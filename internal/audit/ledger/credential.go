package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"sensor-calibration-release/internal/domain/calibration"
)

type frozenMaterial struct {
	Batch              *calibration.CalibrationBatch    `json:"batch"`
	Sensors            []*calibration.SensorRevision    `json:"sensors"`
	Profile            *calibration.CalibrationProfile  `json:"profile"`
	Measurements       []*calibration.MeasurementSet    `json:"measurements"`
	Findings           []*calibration.ReviewFinding     `json:"findings"`
	RecalibrationTasks []*calibration.RecalibrationTask `json:"recalibrationTasks"`
	Reviews            []calibration.ReviewRecord       `json:"reviews"`
}

func FrozenDigest(s *calibration.Snapshot, batchID string) (string, []string, error) {
	return frozenDigestAtVersion(s, batchID, 0)
}

func frozenDigestAtVersion(s *calibration.Snapshot, batchID string, batchVersion int64) (string, []string, error) {
	b, err := s.Batch(batchID)
	if err != nil {
		return "", nil, err
	}
	if b.Status != calibration.StatusFrozen && b.Status != calibration.StatusReleased {
		return "", nil, calibration.Conflict("批次尚未冻结")
	}
	copyBatch := *b
	copyBatch.Status = calibration.StatusFrozen
	if batchVersion > 0 {
		copyBatch.Version = batchVersion
	}
	material := frozenMaterial{Batch: &copyBatch, Profile: s.Profiles[b.ProfileID]}
	for _, sensor := range s.CurrentSensors(batchID) {
		material.Sensors = append(material.Sensors, sensor)
	}
	for _, set := range s.Measurements {
		if set.BatchID == batchID {
			material.Measurements = append(material.Measurements, set)
		}
	}
	for _, finding := range s.Findings {
		if finding.BatchID == batchID {
			material.Findings = append(material.Findings, finding)
		}
	}
	for _, task := range s.RecalibrationTasks {
		if task.BatchID == batchID {
			material.RecalibrationTasks = append(material.RecalibrationTasks, task)
		}
	}
	for _, review := range s.Reviews {
		if review.BatchID == batchID {
			material.Reviews = append(material.Reviews, review)
		}
	}
	sort.Slice(material.Sensors, func(i, j int) bool { return material.Sensors[i].ID < material.Sensors[j].ID })
	sort.Slice(material.Measurements, func(i, j int) bool { return material.Measurements[i].ID < material.Measurements[j].ID })
	sort.Slice(material.Findings, func(i, j int) bool { return material.Findings[i].ID < material.Findings[j].ID })
	sort.Slice(material.RecalibrationTasks, func(i, j int) bool { return material.RecalibrationTasks[i].ID < material.RecalibrationTasks[j].ID })
	ids := make([]string, len(material.Sensors))
	for i, sensor := range material.Sensors {
		ids[i] = sensor.ID
	}
	bts, err := json.Marshal(material)
	if err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(bts)
	return hex.EncodeToString(h[:]), ids, nil
}

func VerifyCredential(s *calibration.Snapshot, credential *calibration.ReleaseCredential) error {
	batch, err := s.Batch(credential.BatchID)
	if err != nil {
		return err
	}
	if batch.Status == calibration.StatusReleased && batch.Version != credential.BatchVersion+1 {
		return calibration.Conflict("凭据批次版本与已放行投影不一致")
	}
	digest, ids, err := frozenDigestAtVersion(s, credential.BatchID, credential.BatchVersion)
	if err != nil {
		return err
	}
	if digest != credential.ContentDigest {
		return calibration.Conflict("凭据摘要与冻结批次内容不一致")
	}
	if len(ids) != len(credential.SensorRevisionIDs) {
		return calibration.Conflict("凭据传感器修订集合不一致")
	}
	for i := range ids {
		if ids[i] != credential.SensorRevisionIDs[i] {
			return calibration.Conflict("凭据传感器修订集合不一致")
		}
	}
	return nil
}
