package workflow

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"sensor-calibration-release/internal/domain/calibration"
)

type BatchQueueFilter struct {
	StationCode string
	Status      calibration.BatchStatus
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Limit       int
	Cursor      string
}

type BatchQueueItem struct {
	Batch          *calibration.CalibrationBatch `json:"batch"`
	CurrentSensors int                           `json:"currentSensors"`
	CompletePoints int                           `json:"completePoints"`
	OpenFindings   int                           `json:"openFindings"`
	NextAction     string                        `json:"nextAction"`
	Blockers       []ReadinessBlocker            `json:"blockers"`
}

type BatchQueueSummary struct {
	Total         int            `json:"total"`
	ByStatus      map[string]int `json:"byStatus"`
	PendingReview int            `json:"pendingReview"`
	PendingIssue  int            `json:"pendingIssue"`
}

type BatchQueueView struct {
	Items      []BatchQueueItem  `json:"items"`
	NextCursor string            `json:"nextCursor,omitempty"`
	Summary    BatchQueueSummary `json:"summary"`
}

type batchCursor struct {
	CreatedAt    time.Time `json:"createdAt"`
	BatchID      string    `json:"batchID"`
	FilterDigest string    `json:"filterDigest"`
}

func (s *Service) ListBatches(filter BatchQueueFilter) (BatchQueueView, error) {
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return BatchQueueView{}, calibration.Validation("limit 必须在 1 到 100 之间")
	}
	if filter.Status != "" && !validQueueStatus(filter.Status) {
		return BatchQueueView{}, calibration.Validation("status 值无效")
	}
	if filter.CreatedFrom != nil && filter.CreatedTo != nil && filter.CreatedTo.Before(*filter.CreatedFrom) {
		return BatchQueueView{}, calibration.Validation("createdTo 不能早于 createdFrom")
	}
	filterDigest := queueFilterDigest(filter)
	var cursor *batchCursor
	if filter.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil || json.Unmarshal(decoded, &cursor) != nil || cursor == nil || cursor.BatchID == "" || cursor.CreatedAt.IsZero() {
			return BatchQueueView{}, calibration.Validation("cursor 无效")
		}
		if cursor.FilterDigest != filterDigest {
			return BatchQueueView{}, calibration.Validation("cursor 与当前筛选条件不匹配")
		}
	}
	snapshot, batches, err := s.store.BatchReadSnapshot()
	if err != nil {
		return BatchQueueView{}, err
	}
	view := BatchQueueView{Items: make([]BatchQueueItem, 0), Summary: BatchQueueSummary{ByStatus: zeroStatusCounts()}}
	matched := make([]*calibration.CalibrationBatch, 0)
	for _, batch := range batches {
		if filter.StationCode != "" && batch.StationCode != filter.StationCode {
			continue
		}
		if filter.Status != "" && batch.Status != filter.Status {
			continue
		}
		if filter.CreatedFrom != nil && batch.CreatedAt.Before(*filter.CreatedFrom) {
			continue
		}
		if filter.CreatedTo != nil && batch.CreatedAt.After(*filter.CreatedTo) {
			continue
		}
		matched = append(matched, batch)
		view.Summary.Total++
		view.Summary.ByStatus[string(batch.Status)]++
		if batch.Status == calibration.StatusReadyReview {
			view.Summary.PendingReview++
		}
		if batch.Status == calibration.StatusFrozen {
			view.Summary.PendingIssue++
		}
	}
	start := 0
	if cursor != nil {
		start = sort.Search(len(matched), func(i int) bool {
			batch := matched[i]
			return batch.CreatedAt.Before(cursor.CreatedAt) || (batch.CreatedAt.Equal(cursor.CreatedAt) && batch.ID > cursor.BatchID)
		})
	}
	end := start + filter.Limit
	if end > len(matched) {
		end = len(matched)
	}
	for _, batch := range matched[start:end] {
		readiness, err := BuildReadiness(snapshot, batch.ID)
		if err != nil {
			return BatchQueueView{}, err
		}
		view.Items = append(view.Items, BatchQueueItem{Batch: batch, CurrentSensors: readiness.CurrentSensors, CompletePoints: readiness.CompletePoints, OpenFindings: readiness.OpenFindings, NextAction: nextBatchAction(batch, readiness), Blockers: readiness.Blockers})
	}
	if end < len(matched) && end > start {
		last := matched[end-1]
		encoded, _ := json.Marshal(batchCursor{CreatedAt: last.CreatedAt, BatchID: last.ID, FilterDigest: filterDigest})
		view.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	return view, nil
}

func queueFilterDigest(filter BatchQueueFilter) string {
	material := struct {
		StationCode string                  `json:"stationCode"`
		Status      calibration.BatchStatus `json:"status"`
		CreatedFrom *time.Time              `json:"createdFrom"`
		CreatedTo   *time.Time              `json:"createdTo"`
	}{filter.StationCode, filter.Status, filter.CreatedFrom, filter.CreatedTo}
	b, _ := json.Marshal(material)
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

func validQueueStatus(status calibration.BatchStatus) bool {
	_, ok := zeroStatusCounts()[string(status)]
	return ok
}

func zeroStatusCounts() map[string]int {
	return map[string]int{
		string(calibration.StatusDraft): 0, string(calibration.StatusPlanLocked): 0,
		string(calibration.StatusSampling): 0, string(calibration.StatusFailed): 0,
		string(calibration.StatusReadyReview): 0, string(calibration.StatusReturned): 0,
		string(calibration.StatusFrozen): 0, string(calibration.StatusReleased): 0,
	}
}

func nextBatchAction(batch *calibration.CalibrationBatch, readiness ReadinessReport) string {
	switch batch.Status {
	case calibration.StatusDraft:
		if readiness.CurrentSensors == 0 {
			return "register_sensor"
		}
		return "lock_profile"
	case calibration.StatusPlanLocked, calibration.StatusSampling:
		return "submit_measurements"
	case calibration.StatusFailed:
		return "recalibrate"
	case calibration.StatusReturned:
		if readiness.OpenFindings > 0 {
			return "recalibrate"
		}
		return "resubmit_review"
	case calibration.StatusReadyReview:
		return "review"
	case calibration.StatusFrozen:
		return "issue_credential"
	default:
		return "query_only"
	}
}
