package calibration

import "time"

func IssueCredential(s *Snapshot, credential *ReleaseCredential, now time.Time) error {
	b, err := s.Batch(credential.BatchID)
	if err != nil {
		return err
	}
	if b.Status != StatusFrozen {
		return Conflict("只有已冻结批次可以签发凭据")
	}
	if s.CredentialForBatch(b.ID) != nil {
		return Conflict("该批次已签发凭据")
	}
	if len(s.BatchFindings(b.ID, true)) > 0 {
		return Validation("存在未闭环问题")
	}
	credential.BatchVersion = b.Version
	credential.Decision = "approved_for_deployment"
	credential.IssuedAt = now.UTC()
	s.Credentials[credential.ID] = credential
	return b.MoveTo(StatusReleased)
}
