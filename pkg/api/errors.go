package api

import (
	"errors"
	"fmt"
)

// Reason classifies an API error so every transport can map it to its own
// status codes and back without losing meaning.
type Reason string

const (
	ReasonBadRequest   Reason = "BadRequest"
	ReasonUnauthorized Reason = "Unauthorized"
	ReasonForbidden    Reason = "Forbidden"
	ReasonNotFound     Reason = "NotFound"
	ReasonConflict     Reason = "Conflict"
	ReasonUnavailable  Reason = "Unavailable"
	ReasonInternal     Reason = "Internal"
)

// Error is the error type returned by every api.Interface implementation.
type Error struct {
	Reason  Reason `json:"reason"`
	Message string `json:"error"`
}

func (e *Error) Error() string { return e.Message }

func newError(reason Reason, format string, args ...any) *Error {
	return &Error{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

func BadRequest(format string, args ...any) error { return newError(ReasonBadRequest, format, args...) }
func Unauthorized(format string, args ...any) error {
	return newError(ReasonUnauthorized, format, args...)
}
func Forbidden(format string, args ...any) error { return newError(ReasonForbidden, format, args...) }
func NotFound(format string, args ...any) error  { return newError(ReasonNotFound, format, args...) }
func Conflict(format string, args ...any) error  { return newError(ReasonConflict, format, args...) }
func Unavailable(format string, args ...any) error {
	return newError(ReasonUnavailable, format, args...)
}
func Internal(format string, args ...any) error { return newError(ReasonInternal, format, args...) }

// ReasonOf returns the reason of err, or ReasonInternal for foreign errors.
func ReasonOf(err error) Reason {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Reason
	}
	return ReasonInternal
}

func IsNotFound(err error) bool     { return err != nil && ReasonOf(err) == ReasonNotFound }
func IsUnauthorized(err error) bool { return err != nil && ReasonOf(err) == ReasonUnauthorized }
func IsForbidden(err error) bool    { return err != nil && ReasonOf(err) == ReasonForbidden }
