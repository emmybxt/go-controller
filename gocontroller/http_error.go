package gocontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Message)
}

func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Message: message}
}

func jsonError(w http.ResponseWriter, status int, message string) error {
	return jsonErrorDetailed(w, status, "", message, nil, "")
}

func jsonErrorDetailed(w http.ResponseWriter, status int, code, message string, details any, traceID string) error {
	w.Header().Set("Content-Type", "application/json")
	errBody := map[string]any{
		"code":    code,
		"message": message,
	}
	if details != nil {
		errBody["details"] = details
	}
	if traceID != "" {
		errBody["trace_id"] = traceID
	}

	return json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   errBody,
	})
}
