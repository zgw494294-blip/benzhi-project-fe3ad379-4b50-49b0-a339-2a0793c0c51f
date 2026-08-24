package evaluation

import (
	"fmt"
	"time"

	"sensor-calibration-release/internal/domain/calibration"
)

type FindingDraft struct {
	Point    float64
	Kind     string
	Severity string
	Message  string
}

func Compare(point float64, stats Statistics, profile *calibration.CalibrationProfile) []FindingDraft {
	findings := make([]FindingDraft, 0, 3)
	if stats.AbsoluteError > profile.AbsoluteTolerance {
		findings = append(findings, FindingDraft{Point: point, Kind: "absolute_error", Severity: "critical", Message: fmt.Sprintf("绝对误差 %.6g 超过阈值 %.6g", stats.AbsoluteError, profile.AbsoluteTolerance)})
	}
	if stats.RelativeError > profile.RelativeTolerance {
		findings = append(findings, FindingDraft{Point: point, Kind: "relative_error", Severity: "critical", Message: fmt.Sprintf("相对误差 %.6g 超过阈值 %.6g", stats.RelativeError, profile.RelativeTolerance)})
	}
	if stats.Spread > profile.RepeatabilityLimit {
		findings = append(findings, FindingDraft{Point: point, Kind: "repeatability", Severity: "critical", Message: fmt.Sprintf("重复性极差 %.6g 超过阈值 %.6g", stats.Spread, profile.RepeatabilityLimit)})
	}
	return findings
}

func Materialize(batchID, revisionID string, drafts []FindingDraft, id func() string, now time.Time) []*calibration.ReviewFinding {
	out := make([]*calibration.ReviewFinding, 0, len(drafts))
	for _, draft := range drafts {
		out = append(out, &calibration.ReviewFinding{ID: id(), BatchID: batchID, SensorRevisionID: revisionID, Point: draft.Point, Kind: draft.Kind, Severity: draft.Severity, Message: draft.Message, Origin: "automatic", Status: calibration.FindingOpen})
	}
	return out
}
