package calibration

import (
	"strconv"
	"strings"
	"time"
)

func ReplaceAutomaticFindings(s *Snapshot, batchID, revisionID string, point float64, findings []*ReviewFinding, now time.Time) []string {
	sensor := s.Sensors[revisionID]
	resolved := make([]string, 0)
	for _, old := range s.Findings {
		if old.BatchID != batchID || old.Status != FindingOpen || old.Point != point {
			continue
		}
		oldSensor := s.Sensors[old.SensorRevisionID]
		if sensor != nil && oldSensor != nil && oldSensor.SensorCode == sensor.SensorCode && oldSensor.Revision < sensor.Revision {
			if len(findings) == 0 {
				t := now.UTC()
				old.Status, old.ResolvedByRevision, old.ReviewedAt = FindingResolved, revisionID, &t
				resolved = append(resolved, old.ID)
			}
		}
	}
	for _, finding := range findings {
		s.Findings[finding.ID] = finding
	}
	return resolved
}

func SetEvaluationStatus(s *Snapshot, batchID string, complete bool) error {
	b, err := s.Batch(batchID)
	if err != nil {
		return err
	}
	if b.Status == StatusReturned {
		b.Version++
		return nil
	}
	next := StatusSampling
	if complete && len(s.BatchFindings(batchID, true)) > 0 {
		next = StatusFailed
	} else if complete {
		next = StatusReadyReview
	}
	if !AllowedTransition(b.Status, next) && b.Status != next {
		return Conflict("批次不能从 %s 转换到 %s", b.Status, next)
	}
	b.Status = next
	b.Version++
	return nil
}

func ReturnReview(s *Snapshot, batchID, reviewer, comment string, findings []*ReviewFinding, now time.Time) error {
	b, err := s.Batch(batchID)
	if err != nil {
		return err
	}
	if b.Status != StatusReadyReview && b.Status != StatusFailed {
		return Conflict("当前状态不能退回复核")
	}
	if b.HasSampler(reviewer) {
		return Forbidden("复核员不能是采样提交人")
	}
	if len(findings) == 0 {
		return Validation("退回复核至少需要一项结构化补正要求")
	}
	profile := s.Profiles[b.ProfileID]
	seen := make(map[string]bool, len(findings))
	ids := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding == nil || finding.BatchID != batchID || !s.IsCurrentRevision(finding.SensorRevisionID, batchID) {
			return Validation("补正要求必须定位到本批次当前传感器修订")
		}
		if profile == nil || !PointInProfile(profile, finding.Point) {
			return Validation("补正要求的标准点不在锁定方案内")
		}
		if strings.TrimSpace(finding.Kind) == "" || strings.TrimSpace(finding.Severity) == "" || strings.TrimSpace(finding.Message) == "" {
			return Validation("补正要求的问题类型、严重级别和说明不能为空")
		}
		key := finding.SensorRevisionID + "\x00" + strconv.FormatFloat(finding.Point, 'g', -1, 64) + "\x00" + finding.Kind + "\x00" + strings.TrimSpace(finding.Message)
		if seen[key] {
			return Validation("补正要求不能重复")
		}
		seen[key] = true
		if finding.ID == "" || s.Findings[finding.ID] != nil {
			return Validation("补正要求标识无效或重复")
		}
		finding.Origin, finding.CreatedBy, finding.Status = "manual", reviewer, FindingOpen
		ids = append(ids, finding.ID)
	}
	for _, finding := range findings {
		s.Findings[finding.ID] = finding
	}
	s.Reviews = append(s.Reviews, ReviewRecord{BatchID: batchID, Reviewer: reviewer, Decision: "returned", Comment: comment, FindingIDs: ids, At: now.UTC()})
	return b.MoveTo(StatusReturned)
}

func ResubmitReview(s *Snapshot, batchID, actor, comment string, now time.Time) error {
	b, err := s.Batch(batchID)
	if err != nil {
		return err
	}
	if b.Status != StatusReturned {
		return Conflict("只有已退回批次可以再次送审")
	}
	if len(s.BatchFindings(batchID, true)) > 0 {
		return Validation("仍有未闭环问题，不能再次送审")
	}
	s.Reviews = append(s.Reviews, ReviewRecord{BatchID: batchID, Reviewer: actor, Decision: "resubmitted", Comment: comment, At: now.UTC()})
	return b.MoveTo(StatusReadyReview)
}

func ApproveReview(s *Snapshot, batchID, reviewer, comment string, now time.Time) error {
	b, err := s.Batch(batchID)
	if err != nil {
		return err
	}
	if len(s.BatchFindings(batchID, true)) > 0 {
		return Validation("仍有未闭环问题，不能通过复核")
	}
	if b.Status != StatusReadyReview {
		return Conflict("只有待复核批次可以通过")
	}
	s.Reviews = append(s.Reviews, ReviewRecord{BatchID: batchID, Reviewer: reviewer, Decision: "approved", Comment: comment, At: now.UTC()})
	return b.Freeze(reviewer, now)
}
