package ledger

import (
	"encoding/json"
	"time"
)

type Event struct {
	SchemaVersion    int             `json:"schemaVersion"`
	Sequence         int64           `json:"sequence"`
	AggregateID      string          `json:"aggregateID"`
	AggregateVersion int64           `json:"aggregateVersion"`
	Type             string          `json:"type"`
	Actor            string          `json:"actor"`
	At               time.Time       `json:"at"`
	IdempotencyKey   string          `json:"idempotencyKey,omitempty"`
	RequestDigest    string          `json:"requestDigest,omitempty"`
	PreviousDigest   string          `json:"previousDigest,omitempty"`
	Projection       json.RawMessage `json:"projection"`
	Response         json.RawMessage `json:"response,omitempty"`
	Digest           string          `json:"digest"`
}

type eventHashMaterial struct {
	SchemaVersion    int             `json:"schemaVersion"`
	Sequence         int64           `json:"sequence"`
	AggregateID      string          `json:"aggregateID"`
	AggregateVersion int64           `json:"aggregateVersion"`
	Type             string          `json:"type"`
	Actor            string          `json:"actor"`
	At               time.Time       `json:"at"`
	IdempotencyKey   string          `json:"idempotencyKey,omitempty"`
	RequestDigest    string          `json:"requestDigest,omitempty"`
	PreviousDigest   string          `json:"previousDigest,omitempty"`
	Projection       json.RawMessage `json:"projection"`
	Response         json.RawMessage `json:"response,omitempty"`
}
