package evaluation

import (
	"sort"
	"sync"

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

type statisticsCacheKey struct {
	SensorRevisionID string
	ReferencePoint   float64
	ReadingTotal     float64
}

type Evaluator struct {
	mu              sync.Mutex
	statisticsCache map[statisticsCacheKey]Statistics
}

func New() *Evaluator {
	return &Evaluator{statisticsCache: make(map[statisticsCacheKey]Statistics)}
}

func (e *Evaluator) ApplyStatistics(set *calibration.MeasurementSet) {
	total := 0.0
	for _, reading := range set.Readings {
		total += reading
	}
	key := statisticsCacheKey{SensorRevisionID: set.SensorRevisionID, ReferencePoint: set.ReferencePoint, ReadingTotal: total}
	e.mu.Lock()
	stats, ok := e.statisticsCache[key]
	if !ok {
		stats = Calculate(set.ReferencePoint, set.Readings)
		e.statisticsCache[key] = stats
	}
	e.mu.Unlock()
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
