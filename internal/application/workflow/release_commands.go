package workflow

import (
	"fmt"
	"strings"

	"sensor-calibration-release/internal/audit/ledger"
	"sensor-calibration-release/internal/domain/calibration"
	"sensor-calibration-release/internal/policy/evaluation"
	"sensor-calibration-release/internal/storage/jsonstore"
)

func (s *Service) Review(batchID string, cmd ReviewCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: batchID}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "review." + cmd.Decision, Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		var err error
		switch cmd.Decision {
		case "approve":
			if len(cmd.Corrections) > 0 {
				return calibration.Validation("通过复核时不能提交补正要求")
			}
			err = calibration.ApproveReview(snapshot, batchID, cmd.Actor, cmd.Comment, s.now())
		case "return":
			findings := make([]*calibration.ReviewFinding, 0, len(cmd.Corrections))
			for _, correction := range cmd.Corrections {
				finding := &calibration.ReviewFinding{ID: s.newID("finding"), BatchID: batchID, SensorRevisionID: correction.SensorRevisionID, Point: correction.ReferencePoint, Kind: correction.ProblemType, Severity: correction.Severity, Message: correction.Description}
				findings = append(findings, finding)
				response.FindingIDs = append(response.FindingIDs, finding.ID)
			}
			err = calibration.ReturnReview(snapshot, batchID, cmd.Actor, cmd.Comment, findings, s.now())
		default:
			return calibration.Validation("decision 必须是 approve 或 return")
		}
		if err != nil {
			return err
		}
		batch := snapshot.Batches[batchID]
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}

func (s *Service) ResubmitReview(batchID string, cmd ResubmitReviewCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	response := MutationResponse{BatchID: batchID}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "review.resubmitted", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		batch, err := snapshot.Batch(batchID)
		if err != nil {
			return err
		}
		if batch.Status != calibration.StatusReturned {
			return calibration.Conflict("只有已退回批次可以再次送审")
		}
		blockers := make([]string, 0)
		coverage, err := evaluation.BuildCoverage(snapshot, batchID)
		if err != nil {
			return err
		}
		for _, sensor := range coverage.Sensors {
			for _, point := range sensor.Points {
				if !point.Complete {
					blockers = append(blockers, fmt.Sprintf("传感器 %s 标准点 %.6g 缺少当前有效证据", sensor.SensorCode, point.ReferencePoint))
				}
			}
		}
		for _, finding := range snapshot.BatchFindings(batchID, true) {
			blockers = append(blockers, fmt.Sprintf("问题 %s 尚未闭环：%s", finding.ID, finding.Message))
		}
		if len(blockers) > 0 {
			return calibration.Validation("再次送审被阻止：%s", strings.Join(blockers, "；"))
		}
		if err := calibration.ResubmitReview(snapshot, batchID, cmd.Actor, cmd.Comment, s.now()); err != nil {
			return err
		}
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}

func (s *Service) Issue(batchID string, cmd IssueCommand) (MutationResponse, error) {
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return MutationResponse{}, calibration.Validation("%v", err)
	}
	fingerprint, err := requestFingerprint(cmd)
	if err != nil {
		return MutationResponse{}, err
	}
	id := s.newID("credential")
	response := MutationResponse{BatchID: batchID, ID: id}
	result, err := s.store.Commit(jsonstore.CommitRequest{AggregateID: batchID, ExpectedVersion: cmd.ExpectedVersion, IdempotencyKey: cmd.IdempotencyKey, RequestDigest: fingerprint, Command: "credential.issued", Actor: cmd.Actor, Response: &response, Mutate: func(snapshot *calibration.Snapshot) error {
		digest, sensorIDs, err := ledger.FrozenDigest(snapshot, batchID)
		if err != nil {
			return err
		}
		credential := &calibration.ReleaseCredential{ID: id, BatchID: batchID, SensorRevisionIDs: sensorIDs, ContentDigest: digest, IssuedBy: cmd.Actor}
		if err := calibration.IssueCredential(snapshot, credential, s.now()); err != nil {
			return err
		}
		batch := snapshot.Batches[batchID]
		response.Version, response.Status = batch.Version, string(batch.Status)
		return nil
	}})
	if err != nil {
		return MutationResponse{}, err
	}
	return decodeMutation(result)
}
