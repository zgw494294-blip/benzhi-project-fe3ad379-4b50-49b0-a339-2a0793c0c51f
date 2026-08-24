package workflow

import (
	"crypto/hmac"
	"sort"
	"time"

	"sensor-calibration-release/internal/audit/ledger"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/policy/evaluation"
)

type BatchView struct {
	Batch        *calibration.CalibrationBatch   `json:"batch"`
	Sensors      []*calibration.SensorRevision   `json:"sensors"`
	Profile      *calibration.CalibrationProfile `json:"profile,omitempty"`
	Measurements []*calibration.MeasurementSet   `json:"measurements"`
	Findings     []*calibration.ReviewFinding    `json:"findings"`
	Reviews      []calibration.ReviewRecord      `json:"reviews"`
	Credential   *calibration.ReleaseCredential  `json:"credential,omitempty"`
	Coverage     evaluation.CoverageReport       `json:"coverage"`
	Readiness    ReadinessReport                 `json:"readiness"`
}

type RecalibrationTaskView struct {
	BatchID          string                           `json:"batchID"`
	SensorRevisionID string                           `json:"sensorRevisionID"`
	Tasks            []*calibration.RecalibrationTask `json:"tasks"`
	RequiredPoints   int                              `json:"requiredPoints"`
	CompletedPoints  int                              `json:"completedPoints"`
	PendingPoints    int                              `json:"pendingPoints"`
	FailedPoints     int                              `json:"failedPoints"`
	InheritedPoints  int                              `json:"inheritedPoints"`
}

type CredentialDevice struct {
	SensorCode string  `json:"sensorCode"`
	Revision   int     `json:"revision"`
	Metric     string  `json:"metric"`
	Unit       string  `json:"unit"`
	RangeMin   float64 `json:"rangeMin"`
	RangeMax   float64 `json:"rangeMax"`
}

type CredentialVerificationFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CredentialSummary struct {
	BatchVersion int64     `json:"batchVersion"`
	Decision     string    `json:"decision"`
	IssuedBy     string    `json:"issuedBy"`
	IssuedAt     time.Time `json:"issuedAt"`
}

type CredentialVerification struct {
	Valid            bool                            `json:"valid"`
	VerifiedAt       time.Time                       `json:"verifiedAt"`
	BatchID          string                          `json:"batchID"`
	CredentialID     string                          `json:"credentialID"`
	CredentialDigest string                          `json:"credentialDigest,omitempty"`
	Credential       *CredentialSummary              `json:"credential,omitempty"`
	Failures         []CredentialVerificationFailure `json:"failures"`
	Devices          []CredentialDevice              `json:"devices"`
}

func (s *Service) GetBatch(batchID string) (BatchView, error) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		return BatchView{}, err
	}
	batch, err := snapshot.Batch(batchID)
	if err != nil {
		return BatchView{}, err
	}
	view := BatchView{Batch: batch, Sensors: snapshot.CurrentSensors(batchID), Profile: snapshot.Profiles[batch.ProfileID], Findings: snapshot.BatchFindings(batchID, false), Credential: snapshot.CredentialForBatch(batchID)}
	if batch.ProfileID != "" {
		view.Coverage, err = evaluation.BuildCoverage(snapshot, batchID)
		if err != nil {
			return BatchView{}, err
		}
	}
	view.Readiness, err = BuildReadiness(snapshot, batchID)
	if err != nil {
		return BatchView{}, err
	}
	for _, set := range snapshot.Measurements {
		if set.BatchID == batchID {
			view.Measurements = append(view.Measurements, set)
		}
	}
	for _, review := range snapshot.Reviews {
		if review.BatchID == batchID {
			view.Reviews = append(view.Reviews, review)
		}
	}
	sort.Slice(view.Sensors, func(i, j int) bool { return view.Sensors[i].SensorCode < view.Sensors[j].SensorCode })
	sort.Slice(view.Measurements, func(i, j int) bool { return view.Measurements[i].CapturedAt.Before(view.Measurements[j].CapturedAt) })
	sort.Slice(view.Findings, func(i, j int) bool { return view.Findings[i].ID < view.Findings[j].ID })
	return view, nil
}

func (s *Service) OpenFindings(batchID string) ([]*calibration.ReviewFinding, error) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		return nil, err
	}
	if _, err := snapshot.Batch(batchID); err != nil {
		return nil, err
	}
	return snapshot.BatchFindings(batchID, true), nil
}

func (s *Service) RecalibrationTasks(batchID, revisionID string) (RecalibrationTaskView, error) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		return RecalibrationTaskView{}, err
	}
	if _, err := snapshot.Batch(batchID); err != nil {
		return RecalibrationTaskView{}, err
	}
	if !snapshot.IsCurrentRevision(revisionID, batchID) {
		return RecalibrationTaskView{}, calibration.Validation("返校任务查询必须使用本批次当前传感器修订")
	}
	sensor := snapshot.Sensors[revisionID]
	if sensor.Revision < 2 {
		return RecalibrationTaskView{}, calibration.Validation("当前传感器修订不是返校修订")
	}
	view := RecalibrationTaskView{BatchID: batchID, SensorRevisionID: revisionID, Tasks: snapshot.RecalibrationTasksForRevision(batchID, revisionID)}
	if len(view.Tasks) == 0 {
		return RecalibrationTaskView{}, calibration.NotFound("当前返校修订没有复验任务")
	}
	sort.Slice(view.Tasks, func(i, j int) bool { return view.Tasks[i].ReferencePoint < view.Tasks[j].ReferencePoint })
	for _, task := range view.Tasks {
		if task.Required {
			view.RequiredPoints++
		}
		switch task.Status {
		case calibration.TaskPassed:
			view.CompletedPoints++
		case calibration.TaskPending:
			view.PendingPoints++
		case calibration.TaskFailed:
			view.FailedPoints++
		case calibration.TaskInherited:
			view.InheritedPoints++
		}
	}
	return view, nil
}

func (s *Service) GetCredential(batchID string) (*calibration.ReleaseCredential, error) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		return nil, err
	}
	if _, err := snapshot.Batch(batchID); err != nil {
		return nil, err
	}
	credential := snapshot.CredentialForBatch(batchID)
	if credential == nil {
		return nil, calibration.NotFound("批次尚未签发凭据")
	}
	if err := ledger.VerifyCredential(snapshot, credential); err != nil {
		return nil, err
	}
	return credential, nil
}

func (s *Service) VerifyCredential(batchID, credentialID, providedDigest string) (CredentialVerification, error) {
	result := CredentialVerification{BatchID: batchID, CredentialID: credentialID, VerifiedAt: s.now().UTC(), Failures: make([]CredentialVerificationFailure, 0), Devices: make([]CredentialDevice, 0)}
	if batchID == "" || credentialID == "" || providedDigest == "" {
		return CredentialVerification{}, calibration.Validation("batchID、credentialID 和 contentDigest 不能为空")
	}
	snapshot, events, err := s.store.ReadState()
	if err != nil {
		return CredentialVerification{}, calibration.Integrity("读取核验快照失败: %v", err)
	}
	if err := ledger.VerifyChain(events); err != nil {
		return CredentialVerification{}, calibration.Integrity("审计账本完整性校验失败: %v", err)
	}
	if _, err := ledger.ValidateProjections(events); err != nil {
		return CredentialVerification{}, calibration.Integrity("审计投影完整性校验失败: %v", err)
	}
	if _, err := snapshot.Batch(batchID); err != nil {
		return CredentialVerification{}, err
	}
	credential := snapshot.Credentials[credentialID]
	if credential == nil {
		result.Failures = append(result.Failures, CredentialVerificationFailure{Code: "credential_not_found", Message: "凭据不存在"})
		return result, nil
	}
	if credential.BatchID != batchID {
		result.Failures = append(result.Failures, CredentialVerificationFailure{Code: "credential_batch_mismatch", Message: "凭据不属于指定批次"})
		return result, nil
	}
	result.CredentialDigest = credential.ContentDigest
	result.Credential = &CredentialSummary{BatchVersion: credential.BatchVersion, Decision: credential.Decision, IssuedBy: credential.IssuedBy, IssuedAt: credential.IssuedAt}
	if err := ledger.VerifyCredential(snapshot, credential); err != nil {
		result.Failures = append(result.Failures, CredentialVerificationFailure{Code: "credential_projection_mismatch", Message: err.Error()})
		return result, nil
	}
	if !hmac.Equal([]byte(credential.ContentDigest), []byte(providedDigest)) {
		result.Failures = append(result.Failures, CredentialVerificationFailure{Code: "content_digest_mismatch", Message: "调用方提供的 contentDigest 与凭据不匹配"})
		return result, nil
	}
	for _, revisionID := range credential.SensorRevisionIDs {
		sensor := snapshot.Sensors[revisionID]
		if sensor == nil {
			return CredentialVerification{}, calibration.Integrity("凭据引用的传感器修订 %s 不存在", revisionID)
		}
		result.Devices = append(result.Devices, CredentialDevice{SensorCode: sensor.SensorCode, Revision: sensor.Revision, Metric: sensor.Metric, Unit: sensor.Unit, RangeMin: sensor.RangeMin, RangeMax: sensor.RangeMax})
	}
	sort.Slice(result.Devices, func(i, j int) bool { return result.Devices[i].SensorCode < result.Devices[j].SensorCode })
	result.Valid = true
	return result, nil
}

func (s *Service) AuditTrail(batchID string) ([]ledger.Event, error) {
	snapshot, err := s.store.Snapshot()
	if err != nil {
		return nil, err
	}
	if _, err := snapshot.Batch(batchID); err != nil {
		return nil, err
	}
	return s.store.Events(batchID), nil
}
