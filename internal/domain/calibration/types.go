package calibration

import "time"

type BatchStatus string

const (
	StatusDraft       BatchStatus = "draft"
	StatusPlanLocked  BatchStatus = "plan_locked"
	StatusSampling    BatchStatus = "sampling"
	StatusFailed      BatchStatus = "failed"
	StatusReadyReview BatchStatus = "ready_review"
	StatusReturned    BatchStatus = "returned"
	StatusFrozen      BatchStatus = "frozen"
	StatusReleased    BatchStatus = "released"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "open"
	FindingResolved FindingStatus = "resolved"
)

type RecalibrationTaskStatus string

const (
	TaskPending   RecalibrationTaskStatus = "pending"
	TaskPassed    RecalibrationTaskStatus = "passed"
	TaskFailed    RecalibrationTaskStatus = "failed"
	TaskInherited RecalibrationTaskStatus = "inherited"
)

type CalibrationBatch struct {
	ID          string      `json:"id"`
	StationCode string      `json:"stationCode"`
	Title       string      `json:"title"`
	Status      BatchStatus `json:"status"`
	SensorIDs   []string    `json:"sensorIDs"`
	ProfileID   string      `json:"profileID,omitempty"`
	Version     int64       `json:"version"`
	CreatedBy   string      `json:"createdBy"`
	CreatedAt   time.Time   `json:"createdAt"`
	FrozenAt    *time.Time  `json:"frozenAt,omitempty"`
	SampledBy   []string    `json:"sampledBy,omitempty"`
	ReviewedBy  string      `json:"reviewedBy,omitempty"`
}

type SensorRevision struct {
	ID                string    `json:"id"`
	BatchID           string    `json:"batchID"`
	SensorCode        string    `json:"sensorCode"`
	Revision          int       `json:"revision"`
	Metric            string    `json:"metric"`
	Unit              string    `json:"unit"`
	RangeMin          float64   `json:"rangeMin"`
	RangeMax          float64   `json:"rangeMax"`
	RecalibrationNote string    `json:"recalibrationNote,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (s SensorRevision) OwnerBatchID() string { return s.BatchID }

type CalibrationProfile struct {
	ID                  string     `json:"id"`
	BatchID             string     `json:"batchID"`
	Points              []float64  `json:"points"`
	RepetitionsPerPoint int        `json:"repetitionsPerPoint"`
	AbsoluteTolerance   float64    `json:"absoluteTolerance"`
	RelativeTolerance   float64    `json:"relativeTolerance"`
	RepeatabilityLimit  float64    `json:"repeatabilityLimit"`
	LockedAt            *time.Time `json:"lockedAt,omitempty"`
}

func (p CalibrationProfile) OwnerBatchID() string { return p.BatchID }

type MeasurementSet struct {
	ID               string    `json:"id"`
	BatchID          string    `json:"batchID"`
	SensorRevisionID string    `json:"sensorRevisionID"`
	ReferencePoint   float64   `json:"referencePoint"`
	Readings         []float64 `json:"readings"`
	Mean             float64   `json:"mean"`
	AbsoluteError    float64   `json:"absoluteError"`
	RelativeError    float64   `json:"relativeError"`
	Spread           float64   `json:"spread"`
	CapturedBy       string    `json:"capturedBy"`
	CapturedAt       time.Time `json:"capturedAt"`
}

func (m MeasurementSet) OwnerBatchID() string { return m.BatchID }

type ReviewFinding struct {
	ID                 string        `json:"id"`
	BatchID            string        `json:"batchID"`
	SensorRevisionID   string        `json:"sensorRevisionID"`
	Point              float64       `json:"point"`
	Kind               string        `json:"kind"`
	Severity           string        `json:"severity"`
	Message            string        `json:"message"`
	Origin             string        `json:"origin"`
	CreatedBy          string        `json:"createdBy,omitempty"`
	Status             FindingStatus `json:"status"`
	ResolvedByRevision string        `json:"resolvedByRevision,omitempty"`
	ReviewedAt         *time.Time    `json:"reviewedAt,omitempty"`
}

func (f ReviewFinding) OwnerBatchID() string { return f.BatchID }

type RecalibrationTask struct {
	ID                    string                  `json:"id"`
	BatchID               string                  `json:"batchID"`
	SensorRevisionID      string                  `json:"sensorRevisionID"`
	ReferencePoint        float64                 `json:"referencePoint"`
	Required              bool                    `json:"required"`
	Status                RecalibrationTaskStatus `json:"status"`
	FindingIDs            []string                `json:"findingIDs,omitempty"`
	SourceRevisionID      string                  `json:"sourceRevisionID,omitempty"`
	EvidenceMeasurementID string                  `json:"evidenceMeasurementID,omitempty"`
	FailureReasons        []string                `json:"failureReasons,omitempty"`
	UpdatedAt             time.Time               `json:"updatedAt"`
}

func (t RecalibrationTask) OwnerBatchID() string { return t.BatchID }

type ReleaseCredential struct {
	ID                string    `json:"id"`
	BatchID           string    `json:"batchID"`
	BatchVersion      int64     `json:"batchVersion"`
	SensorRevisionIDs []string  `json:"sensorRevisionIDs"`
	Decision          string    `json:"decision"`
	ContentDigest     string    `json:"contentDigest"`
	IssuedBy          string    `json:"issuedBy"`
	IssuedAt          time.Time `json:"issuedAt"`
}

func (c ReleaseCredential) OwnerBatchID() string { return c.BatchID }

type ReviewRecord struct {
	BatchID    string    `json:"batchID"`
	Reviewer   string    `json:"reviewer"`
	Decision   string    `json:"decision"`
	Comment    string    `json:"comment,omitempty"`
	FindingIDs []string  `json:"findingIDs,omitempty"`
	At         time.Time `json:"at"`
}
