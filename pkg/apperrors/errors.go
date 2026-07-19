// Package apperrors provides structured error types for the AEGIS platform.
//
// All errors carry an error code, HTTP status mapping, human-readable message,
// and optional structured details. Errors are serializable to RFC 7807
// Problem Details format for consistent API error responses.
//
// Error wrapping is supported via the standard errors.Is/As chain.
package apperrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode represents a machine-readable error classification.
type ErrorCode string

const (
	// ErrCodeNotFound indicates the requested resource does not exist.
	ErrCodeNotFound ErrorCode = "AEGIS_NOT_FOUND"
	// ErrCodeConflict indicates a state conflict (e.g., duplicate, version mismatch).
	ErrCodeConflict ErrorCode = "AEGIS_CONFLICT"
	// ErrCodeValidation indicates input validation failure.
	ErrCodeValidation ErrorCode = "AEGIS_VALIDATION_ERROR"
	// ErrCodeUnauthorized indicates missing or invalid authentication.
	ErrCodeUnauthorized ErrorCode = "AEGIS_UNAUTHORIZED"
	// ErrCodeForbidden indicates insufficient permissions for the action.
	ErrCodeForbidden ErrorCode = "AEGIS_FORBIDDEN"
	// ErrCodeInternal indicates an unexpected internal server error.
	ErrCodeInternal ErrorCode = "AEGIS_INTERNAL_ERROR"
	// ErrCodeBadRequest indicates a malformed or semantically invalid request.
	ErrCodeBadRequest ErrorCode = "AEGIS_BAD_REQUEST"
	// ErrCodeTimeout indicates the operation timed out.
	ErrCodeTimeout ErrorCode = "AEGIS_TIMEOUT"
	// ErrCodeUnavailable indicates the service is temporarily unavailable.
	ErrCodeUnavailable ErrorCode = "AEGIS_UNAVAILABLE"
	// ErrCodeRateLimited indicates the caller has exceeded rate limits.
	ErrCodeRateLimited ErrorCode = "AEGIS_RATE_LIMITED"
	// ErrCodePreconditionFailed indicates a precondition (e.g., If-Match) was not met.
	ErrCodePreconditionFailed ErrorCode = "AEGIS_PRECONDITION_FAILED"
	// ErrCodeInvalidStateTransition indicates an invalid domain state transition.
	ErrCodeInvalidStateTransition ErrorCode = "AEGIS_INVALID_STATE_TRANSITION"
)

// codeToHTTPStatus maps error codes to HTTP status codes.
var codeToHTTPStatus = map[ErrorCode]int{
	ErrCodeNotFound:              http.StatusNotFound,
	ErrCodeConflict:              http.StatusConflict,
	ErrCodeValidation:            http.StatusUnprocessableEntity,
	ErrCodeUnauthorized:          http.StatusUnauthorized,
	ErrCodeForbidden:             http.StatusForbidden,
	ErrCodeInternal:              http.StatusInternalServerError,
	ErrCodeBadRequest:            http.StatusBadRequest,
	ErrCodeTimeout:               http.StatusGatewayTimeout,
	ErrCodeUnavailable:           http.StatusServiceUnavailable,
	ErrCodeRateLimited:           http.StatusTooManyRequests,
	ErrCodePreconditionFailed:    http.StatusPreconditionFailed,
	ErrCodeInvalidStateTransition: http.StatusConflict,
}

// AppError is the canonical error type for the AEGIS platform.
// It implements the error interface and supports JSON serialization
// to RFC 7807 Problem Details format.
type AppError struct {
	// Code is the machine-readable error code.
	Code ErrorCode `json:"code"`
	// Message is the human-readable error message safe for API responses.
	Message string `json:"message"`
	// HTTPStatus is the HTTP status code to return.
	HTTPStatus int `json:"-"`
	// Details contains structured error details (e.g., field-level validation errors).
	Details map[string]interface{} `json:"details,omitempty"`
	// Cause is the underlying error (not serialized to API responses).
	Cause error `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithCause returns a copy of the error with the given underlying cause.
func (e *AppError) WithCause(cause error) *AppError {
	clone := *e
	clone.Cause = cause
	return &clone
}

// WithDetail adds a key-value detail to the error.
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	clone := *e
	if clone.Details == nil {
		clone.Details = make(map[string]interface{})
	}
	clone.Details[key] = value
	return &clone
}

// WithMessage returns a copy of the error with a custom message.
func (e *AppError) WithMessage(msg string) *AppError {
	clone := *e
	clone.Message = msg
	return &clone
}

// WithMessagef returns a copy of the error with a formatted custom message.
func (e *AppError) WithMessagef(format string, args ...interface{}) *AppError {
	clone := *e
	clone.Message = fmt.Sprintf(format, args...)
	return &clone
}

// ProblemDetail converts the error to RFC 7807 Problem Details JSON.
type ProblemDetail struct {
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Status   int                    `json:"status"`
	Detail   string                 `json:"detail"`
	Instance string                 `json:"instance,omitempty"`
	Code     ErrorCode              `json:"code"`
	Errors   map[string]interface{} `json:"errors,omitempty"`
}

// ToProblemDetail converts an AppError to RFC 7807 format.
func (e *AppError) ToProblemDetail(instance string) ProblemDetail {
	return ProblemDetail{
		Type:   fmt.Sprintf("https://aegis.gov.in/errors/%s", e.Code),
		Title:  http.StatusText(e.HTTPStatus),
		Status: e.HTTPStatus,
		Detail: e.Message,
		Code:   e.Code,
		Errors: e.Details,
		Instance: instance,
	}
}

// MarshalJSON implements custom JSON marshaling for API responses.
func (p ProblemDetail) MarshalJSON() ([]byte, error) {
	type Alias ProblemDetail
	return json.Marshal(struct {
		Alias
	}{
		Alias: Alias(p),
	})
}

// --- Constructor Functions ---

// newError creates a new AppError with the given code and message.
func newError(code ErrorCode, message string) *AppError {
	status, ok := codeToHTTPStatus[code]
	if !ok {
		status = http.StatusInternalServerError
	}
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

// NewNotFound creates a not-found error for the given resource.
func NewNotFound(resource string, id interface{}) *AppError {
	return newError(ErrCodeNotFound, fmt.Sprintf("%s with identifier '%v' not found", resource, id))
}

// NewConflict creates a conflict error.
func NewConflict(message string) *AppError {
	return newError(ErrCodeConflict, message)
}

// NewValidation creates a validation error with field-level details.
func NewValidation(message string, fieldErrors map[string]string) *AppError {
	e := newError(ErrCodeValidation, message)
	if fieldErrors != nil {
		e.Details = make(map[string]interface{}, len(fieldErrors))
		for k, v := range fieldErrors {
			e.Details[k] = v
		}
	}
	return e
}

// NewUnauthorized creates an authentication error.
func NewUnauthorized(message string) *AppError {
	return newError(ErrCodeUnauthorized, message)
}

// NewForbidden creates an authorization error.
func NewForbidden(message string) *AppError {
	return newError(ErrCodeForbidden, message)
}

// NewInternal creates an internal server error.
// The cause is logged but not exposed in the API response.
func NewInternal(message string, cause error) *AppError {
	e := newError(ErrCodeInternal, message)
	e.Cause = cause
	return e
}

// NewBadRequest creates a bad request error.
func NewBadRequest(message string) *AppError {
	return newError(ErrCodeBadRequest, message)
}

// NewTimeout creates a timeout error.
func NewTimeout(message string) *AppError {
	return newError(ErrCodeTimeout, message)
}

// NewUnavailable creates a service unavailable error.
func NewUnavailable(message string) *AppError {
	return newError(ErrCodeUnavailable, message)
}

// NewRateLimited creates a rate-limited error.
func NewRateLimited(message string) *AppError {
	return newError(ErrCodeRateLimited, message)
}

// NewInvalidStateTransition creates an error for invalid domain state transitions.
func NewInvalidStateTransition(entity string, from, to string) *AppError {
	return newError(
		ErrCodeInvalidStateTransition,
		fmt.Sprintf("invalid state transition for %s: %s → %s", entity, from, to),
	)
}

// --- Helper Functions ---

// IsAppError checks if the error is an AppError and returns it.
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// IsNotFound checks if the error is a not-found error.
func IsNotFound(err error) bool {
	appErr, ok := IsAppError(err)
	return ok && appErr.Code == ErrCodeNotFound
}

// IsConflict checks if the error is a conflict error.
func IsConflict(err error) bool {
	appErr, ok := IsAppError(err)
	return ok && appErr.Code == ErrCodeConflict
}

// IsValidation checks if the error is a validation error.
func IsValidation(err error) bool {
	appErr, ok := IsAppError(err)
	return ok && appErr.Code == ErrCodeValidation
}

// HTTPStatusFromError extracts the HTTP status code from an error.
// Returns 500 for non-AppError types.
func HTTPStatusFromError(err error) int {
	appErr, ok := IsAppError(err)
	if ok {
		return appErr.HTTPStatus
	}
	return http.StatusInternalServerError
}
