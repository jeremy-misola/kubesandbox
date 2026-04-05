package apierror

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Error represents an API error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Message, e.Detail)
	}
	return e.Message
}

// JSON returns the error as JSON
func (e *Error) JSON() []byte {
	b, _ := json.Marshal(e)
	return b
}

// Common errors
var (
	ErrBadRequest = &Error{
		Code:    http.StatusBadRequest,
		Message: "Bad request",
	}

	ErrUnauthorized = &Error{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
	}

	ErrForbidden = &Error{
		Code:    http.StatusForbidden,
		Message: "Forbidden",
	}

	ErrNotFound = &Error{
		Code:    http.StatusNotFound,
		Message: "Not found",
	}

	ErrConflict = &Error{
		Code:    http.StatusConflict,
		Message: "Conflict",
	}

	ErrInternal = &Error{
		Code:    http.StatusInternalServerError,
		Message: "Internal server error",
	}
)

// NewBadRequest creates a new bad request error
func NewBadRequest(detail string) *Error {
	return &Error{
		Code:    http.StatusBadRequest,
		Message: "Bad request",
		Detail:  detail,
	}
}

// NewUnauthorized creates a new unauthorized error
func NewUnauthorized(detail string) *Error {
	return &Error{
		Code:    http.StatusUnauthorized,
		Message: "Unauthorized",
		Detail:  detail,
	}
}

// NewForbidden creates a new forbidden error
func NewForbidden(detail string) *Error {
	return &Error{
		Code:    http.StatusForbidden,
		Message: "Forbidden",
		Detail:  detail,
	}
}

// NewNotFound creates a new not found error
func NewNotFound(resource, name string) *Error {
	return &Error{
		Code:    http.StatusNotFound,
		Message: "Not found",
		Detail:  fmt.Sprintf("%s %q not found", resource, name),
	}
}

// NewConflict creates a new conflict error
func NewConflict(detail string) *Error {
	return &Error{
		Code:    http.StatusConflict,
		Message: "Conflict",
		Detail:  detail,
	}
}

// NewInternal creates a new internal error
func NewInternal(detail string) *Error {
	return &Error{
		Code:    http.StatusInternalServerError,
		Message: "Internal server error",
		Detail:  detail,
	}
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*Error); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(apiErr.Code)
		w.Write(apiErr.JSON())
		return
	}

	// Generic internal error
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(ErrInternal.JSON())
}
