package jsonstore

import (
	"encoding/json"

	"sensor-calibration-release/internal/domain/calibration"
)

type snapshotFile struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Sequence      int64                 `json:"sequence"`
	LastDigest    string                `json:"lastDigest"`
	Projection    *calibration.Snapshot `json:"projection"`
}

type IdempotentResult struct {
	AggregateID   string          `json:"aggregateID"`
	Command       string          `json:"command"`
	RequestDigest string          `json:"requestDigest,omitempty"`
	Response      json.RawMessage `json:"response"`
}

type CommitRequest struct {
	AggregateID     string
	ExpectedVersion int64
	IdempotencyKey  string
	Command         string
	RequestDigest   string
	Actor           string
	Response        any
	Mutate          func(*calibration.Snapshot) error
}

type CommitResult struct {
	Response json.RawMessage
	Replayed bool
	Version  int64
}
