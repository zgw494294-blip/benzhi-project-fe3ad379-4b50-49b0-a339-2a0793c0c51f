package workflow

type CommandMeta struct {
	Actor           string `json:"actor"`
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type CreateBatchCommand struct {
	CommandMeta
	StationCode string `json:"stationCode"`
	Title       string `json:"title"`
}

type RegisterSensorCommand struct {
	CommandMeta
	SensorCode string  `json:"sensorCode"`
	Metric     string  `json:"metric"`
	Unit       string  `json:"unit"`
	RangeMin   float64 `json:"rangeMin"`
	RangeMax   float64 `json:"rangeMax"`
}

type LockProfileCommand struct {
	CommandMeta
	Points              []float64 `json:"points"`
	RepetitionsPerPoint int       `json:"repetitionsPerPoint"`
	AbsoluteTolerance   float64   `json:"absoluteTolerance"`
	RelativeTolerance   float64   `json:"relativeTolerance"`
	RepeatabilityLimit  float64   `json:"repeatabilityLimit"`
}

type SubmitMeasurementCommand struct {
	CommandMeta
	SensorRevisionID string    `json:"sensorRevisionID"`
	ReferencePoint   float64   `json:"referencePoint"`
	Readings         []float64 `json:"readings"`
}

type MeasurementInput struct {
	ReferencePoint float64   `json:"referencePoint"`
	Readings       []float64 `json:"readings"`
}

type SubmitMeasurementBatchCommand struct {
	CommandMeta
	SensorRevisionID string             `json:"sensorRevisionID"`
	Measurements     []MeasurementInput `json:"measurements"`
}

type RecalibrateCommand struct {
	CommandMeta
	SensorRevisionID string `json:"sensorRevisionID"`
	Note             string `json:"note"`
}

type ReviewCommand struct {
	CommandMeta
	Decision    string            `json:"decision"`
	Comment     string            `json:"comment"`
	Corrections []CorrectionInput `json:"corrections,omitempty"`
}

type CorrectionInput struct {
	SensorRevisionID string  `json:"sensorRevisionID"`
	ReferencePoint   float64 `json:"referencePoint"`
	ProblemType      string  `json:"problemType"`
	Severity         string  `json:"severity"`
	Description      string  `json:"description"`
}

type ResubmitReviewCommand struct {
	CommandMeta
	Comment string `json:"comment,omitempty"`
}

type IssueCommand struct{ CommandMeta }

type MutationResponse struct {
	BatchID             string   `json:"batchID"`
	ID                  string   `json:"id,omitempty"`
	Version             int64    `json:"version"`
	Status              string   `json:"status"`
	Replayed            bool     `json:"replayed,omitempty"`
	PendingRetestPoints int      `json:"pendingRetestPoints,omitempty"`
	FindingIDs          []string `json:"findingIDs,omitempty"`
	ResolvedFindingIDs  []string `json:"resolvedFindingIDs,omitempty"`
}

type MeasurementResult struct {
	ID             string  `json:"id"`
	ReferencePoint float64 `json:"referencePoint"`
	Mean           float64 `json:"mean"`
	AbsoluteError  float64 `json:"absoluteError"`
	RelativeError  float64 `json:"relativeError"`
	Spread         float64 `json:"spread"`
}

type MeasurementBatchResponse struct {
	BatchID             string              `json:"batchID"`
	SensorRevisionID    string              `json:"sensorRevisionID"`
	Measurements        []MeasurementResult `json:"measurements"`
	Version             int64               `json:"version"`
	Status              string              `json:"status"`
	PendingRetestPoints int                 `json:"pendingRetestPoints"`
	ResolvedFindingIDs  []string            `json:"resolvedFindingIDs,omitempty"`
	Replayed            bool                `json:"replayed,omitempty"`
}
