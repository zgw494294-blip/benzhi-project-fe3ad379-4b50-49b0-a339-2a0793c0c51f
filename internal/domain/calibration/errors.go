package calibration

import "fmt"

type ErrorCode string

const (
	CodeValidation ErrorCode = "validation"
	CodeConflict   ErrorCode = "conflict"
	CodeNotFound   ErrorCode = "not_found"
	CodeForbidden  ErrorCode = "forbidden"
	CodeIntegrity  ErrorCode = "integrity"
)

type DomainError struct {
	Code           ErrorCode `json:"code"`
	Message        string    `json:"message"`
	CurrentVersion *int64    `json:"currentVersion,omitempty"`
}

func (e *DomainError) Error() string { return e.Message }

func Validation(format string, args ...any) error {
	return &DomainError{Code: CodeValidation, Message: fmt.Sprintf(format, args...)}
}

func Conflict(format string, args ...any) error {
	return &DomainError{Code: CodeConflict, Message: fmt.Sprintf(format, args...)}
}

func VersionConflict(expected, current int64) error {
	return &DomainError{Code: CodeConflict, Message: fmt.Sprintf("版本冲突：期望 %d，实际 %d", expected, current), CurrentVersion: &current}
}

func NotFound(format string, args ...any) error {
	return &DomainError{Code: CodeNotFound, Message: fmt.Sprintf(format, args...)}
}

func Forbidden(format string, args ...any) error {
	return &DomainError{Code: CodeForbidden, Message: fmt.Sprintf(format, args...)}
}

func Integrity(format string, args ...any) error {
	return &DomainError{Code: CodeIntegrity, Message: fmt.Sprintf(format, args...)}
}
