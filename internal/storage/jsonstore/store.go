package jsonstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"sensor-calibration-release/internal/audit/ledger"
	"sensor-calibration-release/internal/domain/calibration"
)

type Store struct {
	mu           sync.RWMutex
	dir          string
	ledgerPath   string
	snapshotPath string
	projection   *calibration.Snapshot
	events       []ledger.Event
	idempotency  map[string]IdempotentResult
	now          func() time.Time
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	s := &Store{dir: dir, ledgerPath: filepath.Join(dir, "events.jsonl"), snapshotPath: filepath.Join(dir, "snapshot.json"), projection: calibration.NewSnapshot(), idempotency: make(map[string]IdempotentResult), now: time.Now}
	if err := s.recover(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Snapshot() (*calibration.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.projection)
}

func (s *Store) BatchReadSnapshot() (*calibration.Snapshot, []*calibration.CalibrationBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projection, err := cloneSnapshot(s.projection)
	if err != nil {
		return nil, nil, err
	}
	batches := make([]*calibration.CalibrationBatch, 0, len(projection.Batches))
	for _, batch := range projection.Batches {
		batches = append(batches, batch)
	}
	sort.Slice(batches, func(i, j int) bool {
		if batches[i].CreatedAt.Equal(batches[j].CreatedAt) {
			return batches[i].ID < batches[j].ID
		}
		return batches[i].CreatedAt.After(batches[j].CreatedAt)
	})
	return projection, batches, nil
}

func (s *Store) Events(aggregateID string) []ledger.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ledger.Event, 0)
	for _, event := range s.events {
		if aggregateID == "" || event.AggregateID == aggregateID {
			out = append(out, event)
		}
	}
	return out
}

func (s *Store) Commit(req CommitRequest) (CommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.AggregateID == "" || req.Actor == "" || req.Command == "" {
		return CommitResult{}, calibration.Validation("提交缺少批次、操作者或命令")
	}
	if req.IdempotencyKey == "" {
		return CommitResult{}, calibration.Validation("idempotencyKey 不能为空")
	}
	key := req.AggregateID + "\x00" + req.IdempotencyKey
	if old, ok := s.idempotency[key]; ok {
		if old.Command != req.Command {
			return CommitResult{}, calibration.Conflict("idempotencyKey 已用于其他命令")
		}
		if old.RequestDigest != "" && old.RequestDigest != req.RequestDigest {
			return CommitResult{}, calibration.Conflict("idempotencyKey 已用于不同请求内容")
		}
		version := int64(0)
		if b := s.projection.Batches[req.AggregateID]; b != nil {
			version = b.Version
		}
		return CommitResult{Response: append(json.RawMessage(nil), old.Response...), Replayed: true, Version: version}, nil
	}
	current := int64(0)
	if batch := s.projection.Batches[req.AggregateID]; batch != nil {
		current = batch.Version
	}
	if current != req.ExpectedVersion {
		return CommitResult{}, calibration.VersionConflict(req.ExpectedVersion, current)
	}
	next, err := cloneSnapshot(s.projection)
	if err != nil {
		return CommitResult{}, err
	}
	if err := req.Mutate(next); err != nil {
		return CommitResult{}, err
	}
	if err := calibration.ValidateSnapshot(next); err != nil {
		return CommitResult{}, fmt.Errorf("提交投影不满足业务不变量: %w", err)
	}
	batch := next.Batches[req.AggregateID]
	if batch == nil {
		return CommitResult{}, calibration.Validation("提交后批次不存在")
	}
	response, err := json.Marshal(req.Response)
	if err != nil {
		return CommitResult{}, fmt.Errorf("序列化响应: %w", err)
	}
	projection, err := json.Marshal(next)
	if err != nil {
		return CommitResult{}, fmt.Errorf("序列化投影: %w", err)
	}
	previous := ""
	if len(s.events) > 0 {
		previous = s.events[len(s.events)-1].Digest
	}
	event := ledger.Event{SchemaVersion: 1, Sequence: int64(len(s.events) + 1), AggregateID: req.AggregateID, AggregateVersion: batch.Version, Type: req.Command, Actor: req.Actor, At: s.now().UTC(), IdempotencyKey: req.IdempotencyKey, RequestDigest: req.RequestDigest, PreviousDigest: previous, Projection: projection, Response: response}
	if err := ledger.Seal(&event); err != nil {
		return CommitResult{}, err
	}
	if err := s.appendEvent(event); err != nil {
		return CommitResult{}, err
	}
	s.projection = next
	s.events = append(s.events, event)
	s.idempotency[key] = IdempotentResult{AggregateID: req.AggregateID, Command: req.Command, RequestDigest: req.RequestDigest, Response: response}
	if err := s.writeSnapshot(next, event.Sequence, event.Digest); err != nil {
		return CommitResult{}, err
	}
	return CommitResult{Response: response, Version: batch.Version}, nil
}

func (s *Store) ReadState() (*calibration.Snapshot, []ledger.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events, err := s.readEvents()
	if err != nil {
		return nil, nil, fmt.Errorf("读取审计账本: %w", err)
	}
	if len(events) != len(s.events) {
		return nil, nil, fmt.Errorf("磁盘账本事件数量与当前投影位置不一致")
	}
	if len(events) > 0 && events[len(events)-1].Digest != s.events[len(s.events)-1].Digest {
		return nil, nil, fmt.Errorf("磁盘账本摘要与当前投影位置不一致")
	}
	projection, err := cloneSnapshot(s.projection)
	if err != nil {
		return nil, nil, err
	}
	return projection, events, nil
}

func cloneSnapshot(in *calibration.Snapshot) (*calibration.Snapshot, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	out := calibration.NewSnapshot()
	if err := json.Unmarshal(b, out); err != nil {
		return nil, err
	}
	out.EnsureMaps()
	return out, nil
}

func (s *Store) appendEvent(event ledger.Event) error {
	f, err := os.OpenFile(s.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("打开事件账本: %w", err)
	}
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(event); err != nil {
		f.Close()
		return fmt.Errorf("追加事件: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("同步事件账本: %w", err)
	}
	return f.Close()
}

func (s *Store) writeSnapshot(projection *calibration.Snapshot, sequence int64, digest string) error {
	tmp, err := os.CreateTemp(s.dir, "snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("创建快照临时文件: %w", err)
	}
	name := tmp.Name()
	cleanup := func() { tmp.Close(); os.Remove(name) }
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshotFile{SchemaVersion: 1, Sequence: sequence, LastDigest: digest, Projection: projection}); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.snapshotPath); err != nil {
		os.Remove(name)
		return fmt.Errorf("原子替换快照: %w", err)
	}
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func decodeJSONLines(r io.Reader) ([]ledger.Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	events := make([]ledger.Event, 0)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var event ledger.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("事件账本第 %d 行损坏: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

var errSnapshotStale = errors.New("快照与事件账本不一致")
