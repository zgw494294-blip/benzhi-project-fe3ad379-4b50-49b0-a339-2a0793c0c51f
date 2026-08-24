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
		versions[event.AggregateID] = batch.Version
		check.Valid = true
		checks = append(checks, check)
	}
	return checks, nil
}
