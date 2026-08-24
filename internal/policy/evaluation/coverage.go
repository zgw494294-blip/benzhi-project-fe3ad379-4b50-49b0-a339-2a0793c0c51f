package evaluation

import (
	"sort"

	"sensor-calibration-release/internal/domain/calibration"
)

type PointCoverage struct {
	ReferencePoint   float64 `json:"referencePoint"`
	RequiredReadings int     `json:"requiredReadings"`
	ObservedReadings int     `json:"observedReadings"`
	EvidenceRevision string  `json:"evidenceRevision,omitempty"`
	Inherited        bool    `json:"inherited"`
	Complete         bool    `json:"complete"`
}

type SensorCoverage struct {
	SensorCode      string          `json:"sensorCode"`
	CurrentRevision string          `json:"currentRevision"`
	Points          []PointCoverage `json:"points"`
	Complete        bool            `json:"complete"`
}

type CoverageReport struct {
	BatchID  string           `json:"batchID"`
	Sensors  []SensorCoverage `json:"sensors"`
	Complete bool             `json:"complete"`
}

func BuildCoverage(s *calibration.Snapshot, batchID string) (CoverageReport, error) {
	batch, err := s.Batch(batchID)
	if err != nil {
		return CoverageReport{}, err
	}
	profile := s.Profiles[batch.ProfileID]
	if profile == nil {
		return CoverageReport{}, calibration.Validation("批次尚未锁定方案")
	}
	current := s.CurrentSensors(batchID)
	sort.Slice(current, func(i, j int) bool { return current[i].SensorCode < current[j].SensorCode })
	report := CoverageReport{BatchID: batchID, Complete: len(current) > 0}
	for _, sensor := range current {
		coverage := SensorCoverage{SensorCode: sensor.SensorCode, CurrentRevision: sensor.ID, Complete: true}
		evidence := effectiveEvidence(s, sensor)
		for _, point := range profile.Points {
			set := evidence[point]
			item := PointCoverage{ReferencePoint: point, RequiredReadings: profile.RepetitionsPerPoint}
			if set != nil {
				item.ObservedReadings = len(set.Readings)
				item.EvidenceRevision = set.SensorRevisionID
				item.Inherited = set.SensorRevisionID != sensor.ID
				item.Complete = len(set.Readings) == profile.RepetitionsPerPoint
			}
			if !item.Complete {
				coverage.Complete = false
				report.Complete = false
			}
			coverage.Points = append(coverage.Points, item)
		}
		report.Sensors = append(report.Sensors, coverage)
	}
	return report, nil
}

func effectiveEvidence(s *calibration.Snapshot, sensor *calibration.SensorRevision) map[float64]*calibration.MeasurementSet {
	direct := make(map[float64]*calibration.MeasurementSet)
	for _, set := range s.Measurements {
		if set.SensorRevisionID == sensor.ID {
			direct[set.ReferencePoint] = set
		}
	}
	if sensor.Revision == 1 {
		return direct
	}
	problemPoints := unresolvedProblemPoints(s, sensor)
	previous := previousEvidenceByPoint(s, sensor)
	for point, set := range previous {
		if direct[point] == nil && !problemPoints[point] {
			direct[point] = set
		}
	}
	return direct
}

func unresolvedProblemPoints(s *calibration.Snapshot, sensor *calibration.SensorRevision) map[float64]bool {
	points := make(map[float64]bool)
	for _, finding := range s.Findings {
		old := s.Sensors[finding.SensorRevisionID]
		if old == nil || old.SensorCode != sensor.SensorCode || old.Revision >= sensor.Revision {
			continue
		}
		if finding.Status == calibration.FindingOpen {
			points[finding.Point] = true
		}
	}
	return points
}

func previousEvidenceByPoint(s *calibration.Snapshot, sensor *calibration.SensorRevision) map[float64]*calibration.MeasurementSet {
	out := make(map[float64]*calibration.MeasurementSet)
	revisions := make(map[string]int)
	for _, candidate := range s.Sensors {
		if candidate.BatchID == sensor.BatchID && candidate.SensorCode == sensor.SensorCode && candidate.Revision < sensor.Revision {
			revisions[candidate.ID] = candidate.Revision
		}
	}
	for _, set := range s.Measurements {
		revision, ok := revisions[set.SensorRevisionID]
		if !ok {
			continue
		}
		old := out[set.ReferencePoint]
		if old == nil || revisions[old.SensorRevisionID] < revision {
			out[set.ReferencePoint] = set
		}
	}
	return out
}
