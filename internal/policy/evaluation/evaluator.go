package evaluation

import (
	"sort"

	"sensor-calibration-release/internal/domain/calibration"
)

type SensorResult struct {
	Revision *calibration.SensorRevision
	Complete bool
	Drafts   []FindingDraft
}

type BatchResult struct {
	Complete bool
	Sensors  []SensorResult
}

type Evaluator struct{}

func New() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) ApplyStatistics(set *calibration.MeasurementSet) {
	stats := Calculate(set.ReferencePoint, set.Readings)
	set.Mean, set.AbsoluteError, set.RelativeError, set.Spread = stats.Mean, stats.AbsoluteError, stats.RelativeError, stats.Spread
}

func (e *Evaluator) EvaluateRevision(s *calibration.Snapshot, revisionID string) (SensorResult, error) {
	sensor := s.Sensors[revisionID]
	if sensor == nil {
		return SensorResult{}, calibration.NotFound("传感器修订 %s 不存在", revisionID)
	}
	b, err := s.Batch(sensor.BatchID)
	if err != nil {
		return SensorResult{}, err
	}
	profile := s.Profiles[b.ProfileID]
	if profile == nil {
		return SensorResult{}, calibration.Validation("批次尚未锁定方案")
	}
	sets := measurementsForRevision(s, sensor)
	result := SensorResult{Revision: sensor, Complete: true}
	for _, point := range profile.Points {
		set := sets[point]
		if set == nil || len(set.Readings) != profile.RepetitionsPerPoint {
			result.Complete = false
			continue
		}
		result.Drafts = append(result.Drafts, Compare(point, Statistics{Mean: set.Mean, AbsoluteError: set.AbsoluteError, RelativeError: set.RelativeError, Spread: set.Spread}, profile)...)
	}
	return result, nil
}

func (e *Evaluator) EvaluateBatch(s *calibration.Snapshot, batchID string) (BatchResult, error) {
	if _, err := s.Batch(batchID); err != nil {
		return BatchResult{}, err
	}
	current := s.CurrentSensors(batchID)
	sort.Slice(current, func(i, j int) bool { return current[i].SensorCode < current[j].SensorCode })
	result := BatchResult{Complete: len(current) > 0}
	for _, sensor := range current {
		item, err := e.EvaluateRevision(s, sensor.ID)
		if err != nil {
			return BatchResult{}, err
		}
		result.Sensors = append(result.Sensors, item)
		if !item.Complete {
			result.Complete = false
		}
	}
	return result, nil
}

func measurementsForRevision(s *calibration.Snapshot, sensor *calibration.SensorRevision) map[float64]*calibration.MeasurementSet {
	return effectiveEvidence(s, sensor)
}
