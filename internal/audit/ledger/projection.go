package ledger

import (
	"encoding/json"
	"fmt"

	"sensor-calibration-release/internal/domain/calibration"
)

type ProjectionCheck struct {
	Sequence         int64  `json:"sequence"`
	AggregateID      string `json:"aggregateID"`
	AggregateVersion int64  `json:"aggregateVersion"`
	Valid            bool   `json:"valid"`
}

func ValidateProjections(events []Event) ([]ProjectionCheck, error) {
	checks := make([]ProjectionCheck, 0, len(events))
	versions := make(map[string]int64)
	var previous *calibration.Snapshot
	for _, event := range events {
		check := ProjectionCheck{Sequence: event.Sequence, AggregateID: event.AggregateID, AggregateVersion: event.AggregateVersion}
		projection := calibration.NewSnapshot()
		if len(event.Projection) == 0 {
			return checks, fmt.Errorf("审计事件 %d 缺少投影", event.Sequence)
		}
		if err := json.Unmarshal(event.Projection, projection); err != nil {
			return checks, fmt.Errorf("审计事件 %d 投影无法解析: %w", event.Sequence, err)
		}
		projection.EnsureMaps()
		if err := calibration.ValidateSnapshot(projection); err != nil {
			return checks, fmt.Errorf("审计事件 %d 投影无效: %w", event.Sequence, err)
		}
		batch := projection.Batches[event.AggregateID]
		if batch == nil {
			return checks, fmt.Errorf("审计事件 %d 的聚合不存在于投影", event.Sequence)
		}
		if batch.Version != event.AggregateVersion {
			return checks, fmt.Errorf("审计事件 %d 的聚合版本与投影不一致", event.Sequence)
		}
		if previous := versions[event.AggregateID]; previous > 0 && batch.Version <= previous {
			return checks, fmt.Errorf("审计事件 %d 的聚合版本未递增", event.Sequence)
		}
		if event.Actor == "" || event.Type == "" || event.At.IsZero() {
			return checks, fmt.Errorf("审计事件 %d 元数据不完整", event.Sequence)
		}
		if previous != nil {
			if err := assertNoCollateralChange(previous, projection, event.AggregateID, event.Sequence); err != nil {
				return checks, err
			}
		} else {
			if err := assertFirstEventScoped(projection, event.AggregateID, event.Sequence); err != nil {
				return checks, err
			}
		}
		versions[event.AggregateID] = batch.Version
		previous = projection
		check.Valid = true
		checks = append(checks, check)
	}
	return checks, nil
}

// assertNoCollateralChange verifies that an event only mutates data owned by its
// declared aggregate. Any addition, removal or rewrite of data belonging to a
// different aggregate is rejected.
func assertNoCollateralChange(previous, current *calibration.Snapshot, aggregateID string, sequence int64) error {
	previous.EnsureMaps()
	current.EnsureMaps()

	if previous.SchemaVersion != current.SchemaVersion {
		return fmt.Errorf("审计事件 %d 改写了投影 schemaVersion", sequence)
	}

	if err := assertMapScoped(previous.Batches, current.Batches, aggregateID, "批次", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.Sensors, current.Sensors, aggregateID, "传感器", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.Profiles, current.Profiles, aggregateID, "方案", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.Measurements, current.Measurements, aggregateID, "读数集", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.Findings, current.Findings, aggregateID, "问题项", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.RecalibrationTasks, current.RecalibrationTasks, aggregateID, "返校任务", sequence); err != nil {
		return err
	}
	if err := assertOwnedMapScoped(previous.Credentials, current.Credentials, aggregateID, "凭据", sequence); err != nil {
		return err
	}
	if err := assertReviewsScoped(previous.Reviews, current.Reviews, aggregateID, sequence); err != nil {
		return err
	}
	return nil
}

// assertFirstEventScoped ensures the first event cannot bootstrap data for any
// aggregate other than the one it declares.
func assertFirstEventScoped(projection *calibration.Snapshot, aggregateID string, sequence int64) error {
	projection.EnsureMaps()
	for id := range projection.Batches {
		if id != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s", sequence, id)
		}
	}
	for _, sensor := range projection.Sensors {
		if sensor.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的传感器", sequence, sensor.BatchID)
		}
	}
	for _, profile := range projection.Profiles {
		if profile.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的方案", sequence, profile.BatchID)
		}
	}
	for _, set := range projection.Measurements {
		if set.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的读数集", sequence, set.BatchID)
		}
	}
	for _, finding := range projection.Findings {
		if finding.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的问题项", sequence, finding.BatchID)
		}
	}
	for _, task := range projection.RecalibrationTasks {
		if task.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的返校任务", sequence, task.BatchID)
		}
	}
	for _, credential := range projection.Credentials {
		if credential.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的凭据", sequence, credential.BatchID)
		}
	}
	for _, review := range projection.Reviews {
		if review.BatchID != aggregateID {
			return fmt.Errorf("审计事件 %d 引入了非目标聚合 %s 的复核记录", sequence, review.BatchID)
		}
	}
	return nil
}

// ownedEntity is implemented by projection entries that carry an owning BatchID.
type ownedEntity interface {
	OwnerBatchID() string
}

// assertMapScoped compares a map keyed by aggregate id (such as Batches) and
// rejects any change to entries whose key differs from the declared aggregate.
func assertMapScoped[V any](previous, current map[string]*V, aggregateID, label string, sequence int64) error {
	for id := range previous {
		if id != aggregateID {
			if _, ok := current[id]; !ok {
				return fmt.Errorf("审计事件 %d 删除了非目标聚合 %s 的%s", sequence, id, label)
			}
			pb, err := json.Marshal(previous[id])
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化前序%s失败: %w", sequence, label, err)
			}
			cb, err := json.Marshal(current[id])
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化%s失败: %w", sequence, label, err)
			}
			if string(pb) != string(cb) {
				return fmt.Errorf("审计事件 %d 改写了非目标聚合 %s 的%s", sequence, id, label)
			}
		}
	}
	for id := range current {
		if id != aggregateID {
			if _, ok := previous[id]; !ok {
				return fmt.Errorf("审计事件 %d 新增了非目标聚合 %s 的%s", sequence, id, label)
			}
		}
	}
	return nil
}

// assertOwnedMapScoped compares a map of entries that each declare an owning
// BatchID and rejects any change to entries owned by a different aggregate.
func assertOwnedMapScoped[T ownedEntity](previous, current map[string]*T, aggregateID, label string, sequence int64) error {
	for id, entry := range previous {
		if owner := (*entry).OwnerBatchID(); owner != aggregateID {
			cur, ok := current[id]
			if !ok {
				return fmt.Errorf("审计事件 %d 删除了非目标聚合 %s 的%s", sequence, owner, label)
			}
			pb, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化前序%s失败: %w", sequence, label, err)
			}
			cb, err := json.Marshal(cur)
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化%s失败: %w", sequence, label, err)
			}
			if string(pb) != string(cb) {
				return fmt.Errorf("审计事件 %d 改写了非目标聚合 %s 的%s", sequence, owner, label)
			}
		}
	}
	for id, entry := range current {
		if owner := (*entry).OwnerBatchID(); owner != aggregateID {
			if _, ok := previous[id]; !ok {
				return fmt.Errorf("审计事件 %d 新增了非目标聚合 %s 的%s", sequence, owner, label)
			}
		}
	}
	return nil
}

// assertReviewsScoped compares the review slice and rejects any change to
// records owned by a different aggregate.
func assertReviewsScoped(previous, current []calibration.ReviewRecord, aggregateID string, sequence int64) error {
	prevByOwner := make(map[string][]calibration.ReviewRecord)
	for _, review := range previous {
		prevByOwner[review.BatchID] = append(prevByOwner[review.BatchID], review)
	}
	curByOwner := make(map[string][]calibration.ReviewRecord)
	for _, review := range current {
		curByOwner[review.BatchID] = append(curByOwner[review.BatchID], review)
	}
	for owner, prevRecords := range prevByOwner {
		if owner == aggregateID {
			continue
		}
		curRecords := curByOwner[owner]
		if len(prevRecords) != len(curRecords) {
			return fmt.Errorf("审计事件 %d 改写了非目标聚合 %s 的复核记录", sequence, owner)
		}
		for i := range prevRecords {
			pb, err := json.Marshal(prevRecords[i])
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化前序复核记录失败: %w", sequence, err)
			}
			cb, err := json.Marshal(curRecords[i])
			if err != nil {
				return fmt.Errorf("审计事件 %d 序列化复核记录失败: %w", sequence, err)
			}
			if string(pb) != string(cb) {
				return fmt.Errorf("审计事件 %d 改写了非目标聚合 %s 的复核记录", sequence, owner)
			}
		}
	}
	for owner, curRecords := range curByOwner {
		if owner == aggregateID {
			continue
		}
		if len(curRecords) > 0 {
			if _, ok := prevByOwner[owner]; !ok {
				return fmt.Errorf("审计事件 %d 新增了非目标聚合 %s 的复核记录", sequence, owner)
			}
		}
	}
	return nil
}
