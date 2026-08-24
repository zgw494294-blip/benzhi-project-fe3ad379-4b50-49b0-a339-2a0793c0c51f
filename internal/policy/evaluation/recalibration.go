package evaluation

import (
	"sort"
	"time"

	"sensor-calibration-release/internal/domain/calibration"
)

func BuildRecalibrationTasks(s *calibration.Snapshot, revisionID string, newID func() string, now time.Time) ([]*calibration.RecalibrationTask, error) {
	sensor := s.Sensors[revisionID]
	if sensor == nil || sensor.Revision < 2 || !s.IsCurrentRevision(revisionID, sensor.BatchID) {
		return nil, calibration.Validation("复验任务必须属于当前返校修订")
	}
	batch := s.Batches[sensor.BatchID]
	profile := s.Profiles[batch.ProfileID]
	if profile == nil {
		return nil, calibration.Validation("批次尚未锁定方案")
	}
	oldEvidence := previousEvidenceByPoint(s, sensor)
	findingsByPoint := make(map[float64][]string)
	for _, finding := range s.Findings {
		old := s.Sensors[finding.SensorRevisionID]
		if finding.Status == calibration.FindingOpen && old != nil && old.BatchID == sensor.BatchID && old.SensorCode == sensor.SensorCode && old.Revision < sensor.Revision {
			findingsByPoint[finding.Point] = append(findingsByPoint[finding.Point], finding.ID)
		}
	}
	tasks := make([]*calibration.RecalibrationTask, 0, len(profile.Points))
	for _, point := range profile.Points {
		findingIDs := findingsByPoint[point]
		sort.Strings(findingIDs)
		task := &calibration.RecalibrationTask{ID: newID(), BatchID: sensor.BatchID, SensorRevisionID: sensor.ID, ReferencePoint: point, Required: len(findingIDs) > 0, FindingIDs: findingIDs, UpdatedAt: now.UTC()}
		if evidence := oldEvidence[point]; evidence != nil {
			task.SourceRevisionID = evidence.SensorRevisionID
			if !task.Required {
				task.EvidenceMeasurementID = evidence.ID
			}
		}
		if task.Required {
			task.Status = calibration.TaskPending
		} else {
			task.Status = calibration.TaskInherited
		}
		s.RecalibrationTasks[task.ID] = task
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func UpdateRecalibrationTask(s *calibration.Snapshot, set *calibration.MeasurementSet, drafts []FindingDraft, now time.Time) []string {
	for _, task := range s.RecalibrationTasks {
		if task.SensorRevisionID != set.SensorRevisionID || task.ReferencePoint != set.ReferencePoint || !task.Required {
			continue
		}
		task.EvidenceMeasurementID = set.ID
		task.UpdatedAt = now.UTC()
		task.FailureReasons = nil
		if len(drafts) == 0 {
			task.Status = calibration.TaskPassed
			return append([]string(nil), task.FindingIDs...)
		}
		task.Status = calibration.TaskFailed
		for _, draft := range drafts {
			task.FailureReasons = append(task.FailureReasons, draft.Message)
		}
		return nil
	}
	return nil
}
