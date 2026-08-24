package workflow

import (
	"fmt"
	"sort"

	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/policy/evaluation"
)

type ReadinessBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ReadinessReport struct {
	BatchID            string                  `json:"batchID"`
	Status             calibration.BatchStatus `json:"status"`
	CurrentSensors     int                     `json:"currentSensors"`
	RequiredPoints     int                     `json:"requiredPoints"`
	CompletePoints     int                     `json:"completePoints"`
	OpenFindings       int                     `json:"openFindings"`
	CriticalFindings   int                     `json:"criticalFindings"`
	SamplingActors     []string                `json:"samplingActors"`
	CanSubmitSample    bool                    `json:"canSubmitSample"`
	CanRecalibrate     bool                    `json:"canRecalibrate"`
	CanReview          bool                    `json:"canReview"`
	CanIssueCredential bool                    `json:"canIssueCredential"`
	Immutable          bool                    `json:"immutable"`
	Blockers           []ReadinessBlocker      `json:"blockers"`
}

func BuildReadiness(snapshot *calibration.Snapshot, batchID string) (ReadinessReport, error) {
	batch, err := snapshot.Batch(batchID)
	if err != nil {
		return ReadinessReport{}, err
	}
	report := ReadinessReport{BatchID: batchID, Status: batch.Status, CurrentSensors: len(snapshot.CurrentSensors(batchID)), SamplingActors: append([]string(nil), batch.SampledBy...)}
	sort.Strings(report.SamplingActors)
	report.Immutable = batch.Status == calibration.StatusFrozen || batch.Status == calibration.StatusReleased
	report.CanSubmitSample = batch.Status == calibration.StatusPlanLocked || batch.Status == calibration.StatusSampling || batch.Status == calibration.StatusFailed || batch.Status == calibration.StatusReturned
	report.CanRecalibrate = batch.Status == calibration.StatusFailed || batch.Status == calibration.StatusReturned
	report.CanIssueCredential = batch.Status == calibration.StatusFrozen && snapshot.CredentialForBatch(batchID) == nil
	if report.CurrentSensors == 0 {
		report.addBlocker("no_sensors", "批次尚未登记传感器")
	}
	if batch.ProfileID == "" {
		report.addBlocker("profile_unlocked", "采样方案尚未锁定")
		return report, nil
	}
	coverage, err := evaluation.BuildCoverage(snapshot, batchID)
	if err != nil {
		return ReadinessReport{}, err
	}
	for _, sensor := range coverage.Sensors {
		for _, point := range sensor.Points {
			report.RequiredPoints++
			if point.Complete {
				report.CompletePoints++
			} else {
				report.addBlocker("missing_measurement", fmt.Sprintf("传感器 %s 的标准点 %.6g 缺少完整重复读数", sensor.SensorCode, point.ReferencePoint))
			}
		}
	}
	for _, finding := range snapshot.BatchFindings(batchID, true) {
		report.OpenFindings++
		if finding.Severity == "critical" {
			report.CriticalFindings++
		}
	}
	if report.OpenFindings > 0 {
		report.addBlocker("open_findings", fmt.Sprintf("仍有 %d 个未闭环问题", report.OpenFindings))
	}
	if len(report.SamplingActors) == 0 {
		report.addBlocker("no_sampling_actor", "尚无采样提交人，无法校验职责分离")
	}
	report.CanReview = coverage.Complete && report.OpenFindings == 0 && batch.Status == calibration.StatusReadyReview
	if batch.Status == calibration.StatusFrozen {
		report.Blockers = nil
	}
	if batch.Status == calibration.StatusReleased {
		report.Blockers = nil
		report.CanIssueCredential = false
	}
	return report, nil
}

func (r *ReadinessReport) addBlocker(code, message string) {
	for _, blocker := range r.Blockers {
		if blocker.Code == code && blocker.Message == message {
			return
		}
	}
	r.Blockers = append(r.Blockers, ReadinessBlocker{Code: code, Message: message})
}

func (r ReadinessReport) ReviewerEligible(actor string) bool {
	if actor == "" || !r.CanReview {
		return false
	}
	for _, sampler := range r.SamplingActors {
		if sampler == actor {
			return false
		}
	}
	return true
}
