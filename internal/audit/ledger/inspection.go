package ledger

import (
	"sort"
	"time"
)

type AggregateInspection struct {
	AggregateID   string    `json:"aggregateID"`
	EventCount    int       `json:"eventCount"`
	FirstSequence int64     `json:"firstSequence"`
	LastSequence  int64     `json:"lastSequence"`
	LastVersion   int64     `json:"lastVersion"`
	LastEventAt   time.Time `json:"lastEventAt"`
}

type Inspection struct {
	EventCount     int                   `json:"eventCount"`
	AggregateCount int                   `json:"aggregateCount"`
	FirstDigest    string                `json:"firstDigest,omitempty"`
	LastDigest     string                `json:"lastDigest,omitempty"`
	FirstEventAt   *time.Time            `json:"firstEventAt,omitempty"`
	LastEventAt    *time.Time            `json:"lastEventAt,omitempty"`
	EventsByType   map[string]int        `json:"eventsByType"`
	EventsByActor  map[string]int        `json:"eventsByActor"`
	Aggregates     []AggregateInspection `json:"aggregates"`
}

func Inspect(events []Event) Inspection {
	result := Inspection{EventCount: len(events), EventsByType: make(map[string]int), EventsByActor: make(map[string]int)}
	byAggregate := make(map[string]*AggregateInspection)
	for i := range events {
		event := events[i]
		result.EventsByType[event.Type]++
		result.EventsByActor[event.Actor]++
		aggregate := byAggregate[event.AggregateID]
		if aggregate == nil {
			aggregate = &AggregateInspection{AggregateID: event.AggregateID, FirstSequence: event.Sequence}
			byAggregate[event.AggregateID] = aggregate
		}
		aggregate.EventCount++
		aggregate.LastSequence = event.Sequence
		aggregate.LastVersion = event.AggregateVersion
		aggregate.LastEventAt = event.At
		if i == 0 {
			first := event.At
			result.FirstDigest = event.Digest
			result.FirstEventAt = &first
		}
		if i == len(events)-1 {
			last := event.At
			result.LastDigest = event.Digest
			result.LastEventAt = &last
		}
	}
	result.AggregateCount = len(byAggregate)
	for _, aggregate := range byAggregate {
		result.Aggregates = append(result.Aggregates, *aggregate)
	}
	sort.Slice(result.Aggregates, func(i, j int) bool { return result.Aggregates[i].AggregateID < result.Aggregates[j].AggregateID })
	return result
}
