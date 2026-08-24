package calibration

import (
	"math"
	"sort"
	"time"
)

func LockProfile(s *Snapshot, p *CalibrationProfile, now time.Time) error {
	b, err := s.Batch(p.BatchID)
	if err != nil {
		return err
	}
	if b.Status != StatusDraft {
		return Conflict("只能为草稿批次锁定方案")
	}
	if len(b.SensorIDs) == 0 {
		return Validation("锁定方案前至少登记一个传感器")
	}
	if len(p.Points) < 2 {
		return Validation("标准点至少需要两个")
	}
	if p.RepetitionsPerPoint < 2 {
		return Validation("每个标准点至少重复两次")
	}
	if p.AbsoluteTolerance < 0 || p.RelativeTolerance < 0 || p.RepeatabilityLimit < 0 {
		return Validation("阈值不能为负数")
	}
	points := append([]float64(nil), p.Points...)
	sort.Float64s(points)
	for i, point := range points {
		if math.IsNaN(point) || math.IsInf(point, 0) {
			return Validation("标准点必须是有限数值")
		}
		if i > 0 && point == points[i-1] {
			return Validation("标准点不能重复")
		}
	}
	for _, sensor := range s.CurrentSensors(p.BatchID) {
		if points[0] < sensor.RangeMin || points[len(points)-1] > sensor.RangeMax {
			return Validation("标准点未被传感器 %s 的量程覆盖", sensor.SensorCode)
		}
	}
	t := now.UTC()
	p.Points, p.LockedAt = points, &t
	s.Profiles[p.ID] = p
	b.ProfileID = p.ID
	return b.MoveTo(StatusPlanLocked)
}

func PointInProfile(p *CalibrationProfile, point float64) bool {
	for _, candidate := range p.Points {
		if candidate == point {
			return true
		}
	}
	return false
}
