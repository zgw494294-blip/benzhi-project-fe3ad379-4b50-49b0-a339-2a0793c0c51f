package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type selfcheckClient struct {
	base     string
	client   *http.Client
	batchID  string
	version  int64
	sensorID string
}

type mutationResult struct {
	BatchID string `json:"batchID"`
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"version"`
}

func RunSelfcheck(ctx context.Context, listener net.Listener, server *http.Server) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	shutdown := func() {
		_ = Shutdown(server)
		select {
		case err := <-serveErr:
			if err != nil && err != http.ErrServerClosed {
				return
			}
		case <-ctx.Done():
		}
	}
	c := &selfcheckClient{base: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 3 * time.Second}}
	if err := c.waitHealthy(ctx); err != nil {
		shutdown()
		return err
	}
	if err := c.run(ctx); err != nil {
		shutdown()
		return err
	}
	if err := Shutdown(server); err != nil {
		select {
		case <-serveErr:
		case <-ctx.Done():
		}
		return err
	}
	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (c *selfcheckClient) waitHealthy(ctx context.Context) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, _, err := c.call(ctx, http.MethodGet, "/healthz", nil)
		if err == nil && status == http.StatusOK {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待健康检查: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *selfcheckClient) run(ctx context.Context) error {
	created, err := c.mutate(ctx, http.MethodPost, "/v1/batches", map[string]any{"actor": "tech-a", "expectedVersion": 0, "idempotencyKey": "create-1", "stationCode": "CN-SH-017", "title": "PM2.5 传感器部署前校准"}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.batchID, c.version = created.BatchID, created.Version
	sensor, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/sensors", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "sensor-1", "sensorCode": "PM25-A01", "metric": "PM2.5", "unit": "ug/m3", "rangeMin": 0, "rangeMax": 500}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.sensorID, c.version = sensor.ID, sensor.Version
	locked, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/profile:lock", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "profile-1", "points": []float64{0, 100}, "repetitionsPerPoint": 3, "absoluteTolerance": 2, "relativeTolerance": 0.03, "repeatabilityLimit": 2}, http.StatusOK)
	if err != nil {
		return err
	}
	c.version = locked.Version
	firstVersion := c.version
	bad, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/measurements", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "measure-bad", "sensorRevisionID": c.sensorID, "referencePoint": 0, "readings": []float64{5, 5, 5}}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.version = bad.Version
	if _, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/measurements", map[string]any{"actor": "tech-a", "expectedVersion": firstVersion, "idempotencyKey": "measure-bad", "sensorRevisionID": c.sensorID, "referencePoint": 0, "readings": []float64{5, 5, 5}}, http.StatusCreated); err != nil {
		return fmt.Errorf("幂等重放失败: %w", err)
	}
	good, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/measurements:batch", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "measure-good", "sensorRevisionID": c.sensorID, "measurements": []map[string]any{{"referencePoint": 100, "readings": []float64{99.8, 100, 100.2}}}}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.version = good.Version
	status, _, err := c.call(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/reviews", map[string]any{"actor": "reviewer-b", "expectedVersion": 0, "idempotencyKey": "conflict", "decision": "approve"})
	if err != nil || status != http.StatusConflict {
		return fmt.Errorf("未得到预期版本冲突，status=%d err=%v", status, err)
	}
	revision, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/recalibrations", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "recal-1", "sensorRevisionID": c.sensorID, "note": "完成零点校正并复核接线"}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.sensorID, c.version = revision.ID, revision.Version
	status, _, err = c.call(ctx, http.MethodGet, "/v1/batches/"+c.batchID+"/recalibrations/"+c.sensorID+"/tasks", nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("复验任务查询失败: status=%d err=%v", status, err)
	}
	retest, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/measurements", map[string]any{"actor": "tech-a", "expectedVersion": c.version, "idempotencyKey": "retest-1", "sensorRevisionID": c.sensorID, "referencePoint": 0, "readings": []float64{0.1, 0, 0.2}}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.version = retest.Version
	approved, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/reviews", map[string]any{"actor": "reviewer-b", "expectedVersion": c.version, "idempotencyKey": "review-1", "decision": "approve", "comment": "采样完整，返校问题已闭环"}, http.StatusOK)
	if err != nil {
		return err
	}
	c.version = approved.Version
	issued, err := c.mutate(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/release", map[string]any{"actor": "deployer-c", "expectedVersion": c.version, "idempotencyKey": "release-1"}, http.StatusCreated)
	if err != nil {
		return err
	}
	c.version = issued.Version
	status, credentialData, err := c.call(ctx, http.MethodGet, "/v1/batches/"+c.batchID+"/credential", nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("凭据查询失败: status=%d err=%v", status, err)
	}
	var credential struct {
		ID            string `json:"id"`
		ContentDigest string `json:"contentDigest"`
	}
	if err := json.Unmarshal(credentialData, &credential); err != nil {
		return err
	}
	status, _, err = c.call(ctx, http.MethodPost, "/v1/batches/"+c.batchID+"/credential:verify", map[string]any{"credentialID": credential.ID, "contentDigest": credential.ContentDigest})
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("凭据核验失败: status=%d err=%v", status, err)
	}
	for _, path := range []string{"/v1/batches?stationCode=CN-SH-017&limit=10", "/v1/batches/" + c.batchID, "/v1/batches/" + c.batchID + "/findings", "/v1/batches/" + c.batchID + "/audit"} {
		status, _, err := c.call(ctx, http.MethodGet, path, nil)
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("查询 %s 失败: status=%d err=%v", path, status, err)
		}
	}
	return nil
}

func (c *selfcheckClient) mutate(ctx context.Context, method, path string, body any, expected int) (mutationResult, error) {
	var result mutationResult
	status, data, err := c.call(ctx, method, path, body)
	if err != nil {
		return result, err
	}
	if status != expected {
		return result, fmt.Errorf("%s %s 返回 %d，期望 %d: %s", method, path, status, expected, string(data))
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *selfcheckClient) call(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, data, err
}
