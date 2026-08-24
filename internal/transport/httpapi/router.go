package httpapi

import (
	"net/http"

	"sensor-calibration-release/internal/application/workflow"
)

type API struct{ service *workflow.Service }

func New(service *workflow.Service) *API { return &API{service: service} }

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.Health)
	mux.HandleFunc("POST /v1/batches", a.CreateBatch)
	mux.HandleFunc("GET /v1/batches", a.ListBatches)
	mux.HandleFunc("GET /v1/batches/{batchID}", a.GetBatch)
	mux.HandleFunc("POST /v1/batches/{batchID}/sensors", a.RegisterSensor)
	mux.HandleFunc("POST /v1/batches/{batchID}/profile:lock", a.LockProfile)
	mux.HandleFunc("POST /v1/batches/{batchID}/measurements", a.SubmitMeasurement)
	mux.HandleFunc("POST /v1/batches/{batchID}/measurements:batch", a.SubmitMeasurementBatch)
	mux.HandleFunc("POST /v1/batches/{batchID}/recalibrations", a.Recalibrate)
	mux.HandleFunc("GET /v1/batches/{batchID}/recalibrations/{revisionID}/tasks", a.GetRecalibrationTasks)
	mux.HandleFunc("POST /v1/batches/{batchID}/reviews", a.Review)
	mux.HandleFunc("POST /v1/batches/{batchID}/reviews:resubmit", a.ResubmitReview)
	mux.HandleFunc("POST /v1/batches/{batchID}/release", a.IssueCredential)
	mux.HandleFunc("GET /v1/batches/{batchID}/credential", a.GetCredential)
	mux.HandleFunc("POST /v1/batches/{batchID}/credential:verify", a.VerifyCredential)
	mux.HandleFunc("GET /v1/batches/{batchID}/findings", a.GetFindings)
	mux.HandleFunc("GET /v1/batches/{batchID}/audit", a.GetAudit)
	return requestID(mux)
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
