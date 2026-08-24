package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"sensor-calibration-release/internal/domain/calibration"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	CurrentVersion *int64 `json:"currentVersion,omitempty"`
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, calibration.Validation("请求 JSON 无效: %v", err))
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, calibration.Validation("请求只能包含一个 JSON 对象"))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal"
	publicInternal := false
	var domainErr *calibration.DomainError
	if errors.As(err, &domainErr) {
		code = string(domainErr.Code)
		switch domainErr.Code {
		case calibration.CodeValidation:
			status = http.StatusBadRequest
		case calibration.CodeConflict:
			status = http.StatusConflict
		case calibration.CodeNotFound:
			status = http.StatusNotFound
		case calibration.CodeForbidden:
			status = http.StatusForbidden
		case calibration.CodeIntegrity:
			status = http.StatusInternalServerError
			publicInternal = true
		}
	}
	message := err.Error()
	if status == http.StatusInternalServerError && !publicInternal {
		message = "服务内部错误"
	}
	response := apiError{Code: code, Message: message}
	if domainErr != nil {
		response.CurrentVersion = domainErr.CurrentVersion
	}
	writeJSON(w, status, errorBody{Error: response})
}
