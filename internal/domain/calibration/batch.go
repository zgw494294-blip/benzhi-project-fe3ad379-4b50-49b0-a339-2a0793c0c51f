package calibration

import (
	"strings"
	"time"
)

func CreateBatch(id, stationCode, title, actor string, now time.Time) (*CalibrationBatch, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(stationCode) == "" || strings.TrimSpace(title) == "" {
		return nil, Validation("批次 id、站点编码和标题不能为空")
	}
	if strings.TrimSpace(actor) == "" {
		return nil, Validation("操作者不能为空")
	}
	return &CalibrationBatch{ID: id, StationCode: stationCode, Title: title, Status: StatusDraft, Version: 1, CreatedBy: actor, CreatedAt: now.UTC()}, nil
}

func (b *CalibrationBatch) CheckVersion(expected int64) error {
	if expected != b.Version {
		return VersionConflict(expected, b.Version)
	}
	return nil
}

func (b *CalibrationBatch) Mutable() error {
	if b.Status == StatusFrozen || b.Status == StatusReleased {
		return Conflict("批次已冻结，不能修改校准内容")
	}
	return nil
}

func (b *CalibrationBatch) Advance(status BatchStatus) {
	b.Status = status
	b.Version++
}

func (b *CalibrationBatch) AddSampler(actor string) {
	for _, v := range b.SampledBy {
		if v == actor {
			return
		}
	}
	b.SampledBy = append(b.SampledBy, actor)
}

func (b *CalibrationBatch) HasSampler(actor string) bool {
	for _, v := range b.SampledBy {
		if v == actor {
			return true
		}
	}
	return false
}

func (b *CalibrationBatch) Freeze(reviewer string, now time.Time) error {
	if b.Status != StatusReadyReview && b.Status != StatusReturned {
		return Conflict("当前状态 %s 不能冻结", b.Status)
	}
	if b.HasSampler(reviewer) {
		return Forbidden("复核员不能是采样提交人")
	}
	t := now.UTC()
	b.FrozenAt = &t
	b.ReviewedBy = reviewer
	return b.MoveTo(StatusFrozen)
}
