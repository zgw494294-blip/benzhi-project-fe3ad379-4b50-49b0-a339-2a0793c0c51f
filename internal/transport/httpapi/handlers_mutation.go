package httpapi

import (
	"net/http"

	"sensor-calibration-release/internal/application/workflow"
)

func (a *API) Health(w http.ResponseWriter, _ *http.Request) {
	report, err := a.service.Store().Verify()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "storage": report})
}

func (a *API) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.CreateBatchCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.CreateBatch(cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) RegisterSensor(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.RegisterSensorCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.RegisterSensor(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) LockProfile(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.LockProfileCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.LockProfile(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) SubmitMeasurement(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.SubmitMeasurementCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.SubmitMeasurement(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) SubmitMeasurementBatch(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.SubmitMeasurementBatchCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.SubmitMeasurementBatch(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) Recalibrate(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.RecalibrateCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.Recalibrate(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) Review(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ReviewCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.Review(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ResubmitReview(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ResubmitReviewCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.ResubmitReview(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) VerifyCredential(w http.ResponseWriter, r *http.Request) {
	var request struct {
		CredentialID  string `json:"credentialID"`
		ContentDigest string `json:"contentDigest"`
	}
	if !decodeBody(w, r, &request) {
		return
	}
	result, err := a.service.VerifyCredential(r.PathValue("batchID"), request.CredentialID, request.ContentDigest)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) IssueCredential(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.IssueCommand
	if !decodeBody(w, r, &cmd) {
		return
	}
	result, err := a.service.Issue(r.PathValue("batchID"), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
