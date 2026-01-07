package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Error codes for API responses
const (
	ErrCodeInvalidAPIKey      = "INVALID_API_KEY"
	ErrCodeMissingAPIKey      = "MISSING_API_KEY"
	ErrCodeSubscriptionInactive = "SUBSCRIPTION_INACTIVE"
	ErrCodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	ErrCodeInvalidJSON        = "INVALID_JSON"
	ErrCodeValidationError    = "VALIDATION_ERROR"
	ErrCodeInternalError      = "INTERNAL_ERROR"
)

// APIError represents a standardized error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// ErrorResponse wraps an APIError
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// writeError writes a standardized error response
func writeError(w http.ResponseWriter, statusCode int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Fallback to plain text if JSON encoding fails
		http.Error(w, message, statusCode)
	}
}

// writeErrorf is a convenience function for formatted error messages
func writeErrorf(w http.ResponseWriter, statusCode int, code, message string, args ...interface{}) {
	details := fmt.Sprintf(message, args...)
	writeError(w, statusCode, code, message, details)
}

