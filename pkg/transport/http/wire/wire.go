// Package wire is the HTTP mapping shared by the HTTP server and client:
// paths, status codes and the watch stream framing.
package wire

import (
	"encoding/json"
	"net/http"

	"github.com/hasirciogluhq/atellar/pkg/api"
)

const (
	Prefix = "/api/v1"

	// WatchContentType marks a newline-delimited stream of Event frames.
	WatchContentType = "application/json;stream=watch"

	// ErrorEvent is the frame type that ends a watch with an error.
	ErrorEvent = "ERROR"

	// MaxBodyBytes caps request bodies.
	MaxBodyBytes = 1 << 20
)

// Event is one line of a watch stream.
type Event struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object,omitempty"`
	Error  *api.Error      `json:"error,omitempty"`
}

var reasonStatus = map[api.Reason]int{
	api.ReasonBadRequest:   http.StatusBadRequest,
	api.ReasonUnauthorized: http.StatusUnauthorized,
	api.ReasonForbidden:    http.StatusForbidden,
	api.ReasonNotFound:     http.StatusNotFound,
	api.ReasonConflict:     http.StatusConflict,
	api.ReasonUnavailable:  http.StatusServiceUnavailable,
	api.ReasonInternal:     http.StatusInternalServerError,
}

// Status returns the HTTP status for an API error reason.
func Status(reason api.Reason) int {
	if code, ok := reasonStatus[reason]; ok {
		return code
	}
	return http.StatusInternalServerError
}

// Reason returns the API error reason for an HTTP status.
func Reason(status int) api.Reason {
	for reason, code := range reasonStatus {
		if code == status {
			return reason
		}
	}
	return api.ReasonInternal
}
