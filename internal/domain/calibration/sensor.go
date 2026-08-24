package calibration

import (
	"fmt"
	"strings"
	"time"
)

func RegisterSensor(s *Snapshot, batchID, id, code, metric, unit string, min, max float64, now time.Time) (*SensorRevision, error) {
	b, err := s.Batch(batchID)
	if err != nil {
		return nil, err
	}
	if err := b.Mutable(); err != nil {
		return nil, err
	}
	if b.Status != StatusDraft {
		return nil, Conflict("只能在草稿状态登记传感器")
	}
	if strings.TrimSpace(code) == "" || strings.TrimSpace(metric) == "" || strings.TrimSpace(unit) == "" {
		return nil, Validation("传感器编码、指标和单位不能为空")
	}
	if min >= max {
		return nil, Validation("量程下限必须小于上限")
	}
	for _, existing := range s.Sensors {
		if existing.BatchID == batchID && existing.SensorCode == code {
			return nil, Conflict("传感器编码 %s 已登记", code)
		}
	}
	sensor := &SensorRevision{ID: id, BatchID: batchID, SensorCode: code, Revision: 1, Metric: metric, Unit: unit, RangeMin: min, RangeMax: max, CreatedAt: now.UTC()}
	s.Sensors[id] = sensor
	b.SensorIDs = append(b.SensorIDs, id)
	b.Version++
	return sensor, nil
}

func RecalibrateSensor(s *Snapshot, batchID, oldID, newID, note string, now time.Time) (*SensorRevision, error) {
	b, err := s.Batch(batchID)
	if err != nil {
		return nil, err
	}
	if err := b.Mutable(); err != nil {
		return nil, err
	}
	if b.Status != StatusFailed && b.Status != StatusReturned {
		return nil, Conflict("只有不合格或退回批次可以返校")
	}
	old := s.Sensors[oldID]
	if old == nil || old.BatchID != batchID {
		return nil, Validation("传感器修订不属于该批次")
	}
	if !s.IsCurrentRevision(oldID, batchID) {
		return nil, Validation("只能为当前传感器修订创建返校任务")
	}
	if strings.TrimSpace(note) == "" {
		return nil, Validation("返校说明不能为空")
	}
	for _, finding := range s.Findings {
		if finding.SensorRevisionID == oldID && finding.Status == FindingOpen {
			copy := *old
			copy.ID, copy.Revision, copy.RecalibrationNote, copy.CreatedAt = newID, old.Revision+1, note, now.UTC()
			s.Sensors[newID] = &copy
			b.SensorIDs = append(b.SensorIDs, newID)
			if b.Status == StatusReturned {
				b.Version++
			} else if err := b.MoveTo(StatusSampling); err != nil {
				return nil, err
			}
			return &copy, nil
		}
	}
	return nil, Validation("传感器 %s 没有待闭环问题", fmt.Sprint(oldID))
}
