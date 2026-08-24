package jsonstore

import (
	"encoding/json"
	"fmt"
	"os"

	"sensor-calibration-release/internal/audit/ledger"
	"sensor-calibration-release/internal/domain/calibration"
)

func (s *Store) recover() error {
	events, err := s.readEvents()
	if err != nil {
		return err
	}
	if err := ledger.VerifyChain(events); err != nil {
		return fmt.Errorf("校验事件账本: %w", err)
	}
	if _, err := ledger.ValidateProjections(events); err != nil {
		return fmt.Errorf("校验事件投影: %w", err)
	}
	s.events = events
	for _, event := range events {
		if event.IdempotencyKey != "" {
			s.idempotency[event.AggregateID+"\x00"+event.IdempotencyKey] = IdempotentResult{AggregateID: event.AggregateID, Command: event.Type, RequestDigest: event.RequestDigest, Response: append(json.RawMessage(nil), event.Response...)}
		}
	}
	if len(events) == 0 {
		if _, err := os.Stat(s.snapshotPath); err == nil {
			return fmt.Errorf("存在无账本支撑的快照")
		}
		return nil
	}
	projection, snapshotErr := s.readSnapshot(events[len(events)-1])
	if snapshotErr == nil {
		if err := calibration.ValidateSnapshot(projection); err != nil {
			return fmt.Errorf("投影快照不满足业务不变量: %w", err)
		}
		s.projection = projection
		return nil
	}
	last := events[len(events)-1]
	projection = calibration.NewSnapshot()
	if err := json.Unmarshal(last.Projection, projection); err != nil {
		return fmt.Errorf("从事件重放投影: %w", err)
	}
	projection.EnsureMaps()
	if err := calibration.ValidateSnapshot(projection); err != nil {
		return fmt.Errorf("重放投影不满足业务不变量: %w", err)
	}
	if err := s.writeSnapshot(projection, last.Sequence, last.Digest); err != nil {
		return fmt.Errorf("重建投影快照: %w", err)
	}
	s.projection = projection
	return nil
}

func (s *Store) readEvents() ([]ledger.Event, error) {
	f, err := os.Open(s.ledgerPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return decodeJSONLines(f)
}

func (s *Store) readSnapshot(last ledger.Event) (*calibration.Snapshot, error) {
	f, err := os.Open(s.snapshotPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var envelope snapshotFile
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&envelope); err != nil {
		return nil, err
	}
	if envelope.SchemaVersion != 1 || envelope.Sequence != last.Sequence || envelope.LastDigest != last.Digest || envelope.Projection == nil {
		return nil, errSnapshotStale
	}
	envelope.Projection.EnsureMaps()
	return envelope.Projection, nil
}
