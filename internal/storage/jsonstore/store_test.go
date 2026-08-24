package jsonstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func TestIdempotencyConflictAndRecovery(t *testing.T) {
	dir := t.TempDir()
	store, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	cmd := workflow.CreateBatchCommand{CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: 0, IdempotencyKey: "create-key"}, StationCode: "ST-01", Title: "部署前校准"}
	created, err := service.CreateBatch(cmd)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.CreateBatch(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.BatchID != created.BatchID {
		t.Fatalf("创建批次未正确幂等重放: %+v", replayed)
	}
	if replayed.Version != created.Version || replayed.Status != created.Status {
		t.Fatalf("幂等重放应返回原始版本和状态: created=%+v replayed=%+v", created, replayed)
	}
	_, err = service.RegisterSensor(created.BatchID, workflow.RegisterSensorCommand{CommandMeta: workflow.CommandMeta{Actor: "tech", ExpectedVersion: 0, IdempotencyKey: "wrong-version"}, SensorCode: "S-1", Metric: "temperature", Unit: "C", RangeMin: -20, RangeMax: 60})
	var domainErr *calibration.DomainError
	if !errors.As(err, &domainErr) || domainErr.Code != calibration.CodeConflict {
		t.Fatalf("应返回领域冲突，实际 %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := jsonstore.Open(dir)
	if err != nil {
		t.Fatalf("应从有效账本重建损坏快照: %v", err)
	}
	snapshot, err := reopened.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Batches[created.BatchID] == nil {
		t.Fatal("重建投影缺少已创建批次")
	}
	report, err := reopened.Verify()
	if err != nil || !report.Valid || report.EventCount != 1 {
		t.Fatalf("恢复后的完整性报告无效: %+v %v", report, err)
	}
}
