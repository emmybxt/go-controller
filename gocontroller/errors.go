package gocontroller

import (
	"fmt"
	"net/http"
)

// APIError is a standardized error model for HTTP responses.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    any
	Cause      error
}

func (e *APIError) Error() string {
	if e == nil {
		return "api error"
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.Cause }

func NewAPIError(status int, code, message string) *APIError {
	return &APIError{StatusCode: status, Code: code, Message: message}
}

func BadRequestError(message string) *APIError {
	return NewAPIError(http.StatusBadRequest, "bad_request", message)
}

func UnauthorizedError(message string) *APIError {
	return NewAPIError(http.StatusUnauthorized, "unauthorized", message)
}

func ForbiddenError(message string) *APIError {
	return NewAPIError(http.StatusForbidden, "forbidden", message)
}

func NotFoundError(message string) *APIError {
	return NewAPIError(http.StatusNotFound, "not_found", message)
}

func ConflictError(message string) *APIError {
	return NewAPIError(http.StatusConflict, "conflict", message)
}

func InternalError(message string) *APIError {
	return NewAPIError(http.StatusInternalServerError, "internal_error", message)
}
