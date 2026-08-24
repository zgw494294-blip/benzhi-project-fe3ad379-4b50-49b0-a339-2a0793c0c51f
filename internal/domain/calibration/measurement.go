package calibration

import (
	"math"
	"time"
)

func PutMeasurement(s *Snapshot, set *MeasurementSet, now time.Time) error {
	return PutMeasurements(s, []*MeasurementSet{set}, now)
}

func PutMeasurements(s *Snapshot, sets []*MeasurementSet, now time.Time) error {
	if len(sets) == 0 {
		return Validation("批量读数至少包含一个标准点")
	}
	if sets[0] == nil {
		return Validation("批量读数不能包含空项目")
	}
	batchID := sets[0].BatchID
	sensorID := sets[0].SensorRevisionID
	b, err := s.Batch(batchID)
	if err != nil {
		return err
	}
	if err := b.Mutable(); err != nil {
		return err
	}
	if b.Status != StatusPlanLocked && b.Status != StatusSampling && b.Status != StatusFailed && b.Status != StatusReturned {
		return Conflict("当前状态不能提交读数")
	}
	sensor := s.Sensors[sensorID]
	if sensor == nil || sensor.BatchID != batchID {
		return Validation("传感器修订不属于该批次")
	}
	if !s.IsCurrentRevision(sensorID, batchID) {
		return Validation("只能为当前传感器修订提交读数")
	}
	profile := s.Profiles[b.ProfileID]
	seen := make(map[float64]bool, len(sets))
	seenIDs := make(map[string]bool, len(sets))
	for _, set := range sets {
		if set == nil || set.BatchID != batchID || set.SensorRevisionID != sensorID {
			return Validation("批量读数必须属于同一批次和传感器修订")
		}
		if profile == nil || !PointInProfile(profile, set.ReferencePoint) {
			return Validation("标准点不在锁定方案内")
		}
		if seen[set.ReferencePoint] {
			return Validation("批量请求中的标准点不能重复")
		}
		seen[set.ReferencePoint] = true
		if set.ID == "" || seenIDs[set.ID] || s.Measurements[set.ID] != nil {
			return Validation("批量读数标识无效或重复")
		}
		seenIDs[set.ID] = true
		if set.CapturedBy == "" {
			return Validation("采样提交人不能为空")
		}
		if len(set.Readings) != profile.RepetitionsPerPoint {
			return Validation("标准点需要 %d 个重复读数", profile.RepetitionsPerPoint)
		}
		for _, reading := range set.Readings {
			if math.IsNaN(reading) || math.IsInf(reading, 0) || reading < sensor.RangeMin || reading > sensor.RangeMax {
				return Validation("标准点 %.6g 的读数超出传感器量程", set.ReferencePoint)
			}
		}
		for _, existing := range s.Measurements {
			if existing.SensorRevisionID == sensorID && existing.ReferencePoint == set.ReferencePoint {
				return Conflict("该传感器修订的标准点 %.6g 读数已提交且不可变", set.ReferencePoint)
			}
		}
	}
	for _, set := range sets {
		set.CapturedAt = now.UTC()
		s.Measurements[set.ID] = set
		b.AddSampler(set.CapturedBy)
	}
	return nil
}
