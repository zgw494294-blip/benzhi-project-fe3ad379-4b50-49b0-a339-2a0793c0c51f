package jsonstore

import (
	"fmt"

	"sensor-calibration-release/internal/audit/ledger"
)

type IntegrityReport struct {
	EventCount     int            `json:"eventCount"`
	LastDigest     string         `json:"lastDigest,omitempty"`
	AggregateCount int            `json:"aggregateCount"`
	EventsByType   map[string]int `json:"eventsByType"`
	Valid          bool           `json:"valid"`
}

func (s *Store) Verify() (IntegrityReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// 每次完整性检查都以当前磁盘账本为准，而不是内存缓存。这样运行期对
	// events.jsonl 的删除、覆盖、截断或篡改都会被立即发现。
	diskEvents, err := s.readEvents()
	if err != nil {
		return IntegrityReport{}, fmt.Errorf("读取审计账本失败: %w", err)
	}
	if err := ledger.VerifyChain(diskEvents); err != nil {
		return IntegrityReport{EventCount: len(diskEvents)}, fmt.Errorf("审计链无效: %w", err)
	}
	if _, err := ledger.ValidateProjections(diskEvents); err != nil {
		return IntegrityReport{EventCount: len(diskEvents)}, fmt.Errorf("事件投影无效: %w", err)
	}
	// 确认磁盘账本与内存事件位置一致，防止内存投影领先于已失效的底层持久化资源。
	if len(diskEvents) != len(s.events) {
		return IntegrityReport{EventCount: len(diskEvents)}, fmt.Errorf("磁盘账本事件数量与当前投影位置不一致")
	}
	if len(diskEvents) > 0 && diskEvents[len(diskEvents)-1].Digest != s.events[len(s.events)-1].Digest {
		return IntegrityReport{EventCount: len(diskEvents)}, fmt.Errorf("磁盘账本摘要与当前投影位置不一致")
	}
	report := IntegrityReport{EventCount: len(diskEvents)}
	if len(diskEvents) > 0 {
		report.LastDigest = diskEvents[len(diskEvents)-1].Digest
	}
	inspection := ledger.Inspect(diskEvents)
	report.AggregateCount = inspection.AggregateCount
	report.EventsByType = inspection.EventsByType
	report.Valid = true
	return report, nil
}
