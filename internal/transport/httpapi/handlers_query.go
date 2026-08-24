package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"sensor-calibration-release/internal/application/workflow"
	"sensor-calibration-release/internal/domain/calibration"
)

func (a *API) ListBatches(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := workflow.BatchQueueFilter{StationCode: query.Get("stationCode"), Status: calibration.BatchStatus(query.Get("status")), Cursor: query.Get("cursor")}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, calibration.Validation("limit 必须是整数"))
			return
		}
		filter.Limit = limit
	}
	var err error
	if raw := query.Get("createdFrom"); raw != "" {
		value, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			err = calibration.Validation("createdFrom 必须是 RFC3339 时间")
		} else {
			filter.CreatedFrom = &value
		}
	}
	if err == nil {
		if raw := query.Get("createdTo"); raw != "" {
			value, parseErr := time.Parse(time.RFC3339, raw)
			if parseErr != nil {
				err = calibration.Validation("createdTo 必须是 RFC3339 时间")
			} else {
				filter.CreatedTo = &value
			}
		}
	}
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.ListBatches(filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) GetBatch(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.GetBatch(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) GetCredential(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.GetCredential(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) GetRecalibrationTasks(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.RecalibrationTasks(r.PathValue("batchID"), r.PathValue("revisionID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) GetFindings(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.OpenFindings(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": result})
}

func (a *API) GetAudit(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.AuditTrail(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": result})
}
