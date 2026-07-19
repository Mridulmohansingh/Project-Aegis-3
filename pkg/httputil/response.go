// Package httputil provides HTTP response helpers for the AEGIS API layer.
//
// All API responses follow a consistent structure:
//   - Success responses use the standard JSON envelope
//   - Error responses follow RFC 7807 Problem Details
//   - All responses include correlation headers (X-Request-ID)
package httputil

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/aegis-platform/aegis/pkg/apperrors"
)

// MaxBodySize is the maximum allowed request body size (10 MB).
const MaxBodySize = 10 * 1024 * 1024

// SuccessResponse is the standard envelope for successful API responses.
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta,omitempty"`
}

// Meta holds optional metadata for paginated or enriched responses.
type Meta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more,omitempty"`
	TotalCount *int64 `json:"total_count,omitempty"`
}

// RespondJSON writes a JSON response with the given status code and data.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// If encoding fails, we've already written headers, so just log
			// The caller's middleware should catch this via response wrapper
			http.Error(w, `{"error":"internal encoding error"}`, http.StatusInternalServerError)
		}
	}
}

// RespondSuccess writes a successful JSON response with the data wrapped in a SuccessResponse.
func RespondSuccess(w http.ResponseWriter, status int, data interface{}) {
	RespondJSON(w, status, SuccessResponse{Data: data})
}

// RespondSuccessWithPagination writes a paginated success response.
func RespondSuccessWithPagination(w http.ResponseWriter, data interface{}, nextCursor string, hasMore bool) {
	RespondJSON(w, http.StatusOK, SuccessResponse{
		Data: data,
		Meta: &Meta{
			NextCursor: nextCursor,
			HasMore:    hasMore,
		},
	})
}

// RespondError writes an error response in RFC 7807 Problem Details format.
// It accepts either an *apperrors.AppError or a generic error.
func RespondError(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperrors.IsAppError(err)
	if !ok {
		// Unknown error — wrap as internal error, do not expose details
		appErr = apperrors.NewInternal("an unexpected error occurred", err)
	}

	pd := appErr.ToProblemDetail(r.URL.Path)
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(appErr.HTTPStatus)
	json.NewEncoder(w).Encode(pd)
}

// RespondNoContent writes a 204 No Content response.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// BindJSON reads and unmarshals the request body into the given destination.
// It enforces MaxBodySize and returns a user-friendly error on failure.
func BindJSON(r *http.Request, dest interface{}) error {
	if r.Body == nil {
		return apperrors.NewBadRequest("request body is required")
	}

	// Limit body size to prevent DoS
	body := http.MaxBytesReader(nil, r.Body, MaxBodySize)
	defer body.Close()

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		switch {
		case err.Error() == "http: request body too large":
			return apperrors.NewBadRequest("request body exceeds maximum size of 10MB")
		default:
			return apperrors.NewBadRequest("invalid JSON: " + err.Error())
		}
	}

	// Ensure no trailing content
	if decoder.More() {
		return apperrors.NewBadRequest("request body contains multiple JSON values")
	}

	return nil
}

// ReadBody reads the entire request body with a size limit.
func ReadBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, apperrors.NewBadRequest("request body is required")
	}
	body := http.MaxBytesReader(nil, r.Body, MaxBodySize)
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, apperrors.NewBadRequest("failed to read request body: " + err.Error())
	}
	return data, nil
}
