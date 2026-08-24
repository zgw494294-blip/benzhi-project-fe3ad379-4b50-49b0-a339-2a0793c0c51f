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
	report := IntegrityReport{EventCount: len(s.events)}
	if len(s.events) > 0 {
		report.LastDigest = s.events[len(s.events)-1].Digest
	}
	if err := ledger.VerifyChain(s.events); err != nil {
		return report, fmt.Errorf("审计链无效: %w", err)
	}
	if _, err := ledger.ValidateProjections(s.events); err != nil {
		return report, fmt.Errorf("事件投影无效: %w", err)
	}
	inspection := ledger.Inspect(s.events)
	report.AggregateCount = inspection.AggregateCount
	report.EventsByType = inspection.EventsByType
	report.Valid = true
	return report, nil
}
