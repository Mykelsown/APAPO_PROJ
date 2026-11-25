// Package httpx holds shared HTTP helpers: JSON responses, the error shape and middleware.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorBody is the single error shape used by every endpoint.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a stable code, a plain message and optional details.
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// WriteError writes the standard error body. details may be nil.
func WriteError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	WriteJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message, Details: details}})
}
