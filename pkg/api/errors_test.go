package api

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorReasons(t *testing.T) {
	cases := []struct {
		err    error
		reason Reason
	}{
		{BadRequest("bad %d", 1), ReasonBadRequest},
		{Unauthorized("who"), ReasonUnauthorized},
		{Forbidden("no"), ReasonForbidden},
		{NotFound("gone"), ReasonNotFound},
		{Conflict("again"), ReasonConflict},
		{Unavailable("later"), ReasonUnavailable},
		{Internal("oops"), ReasonInternal},
		{errors.New("foreign"), ReasonInternal},
		{fmt.Errorf("wrapped: %w", NotFound("x")), ReasonNotFound},
	}
	for _, tc := range cases {
		t.Run(string(tc.reason)+"/"+tc.err.Error(), func(t *testing.T) {
			if got := ReasonOf(tc.err); got != tc.reason {
				t.Fatalf("ReasonOf = %s, want %s", got, tc.reason)
			}
			if got, want := IsNotFound(tc.err), tc.reason == ReasonNotFound; got != want {
				t.Errorf("IsNotFound = %v", got)
			}
			if got, want := IsUnauthorized(tc.err), tc.reason == ReasonUnauthorized; got != want {
				t.Errorf("IsUnauthorized = %v", got)
			}
			if got, want := IsForbidden(tc.err), tc.reason == ReasonForbidden; got != want {
				t.Errorf("IsForbidden = %v", got)
			}
		})
	}
	if BadRequest("bad %d", 1).Error() != "bad 1" {
		t.Error("message not formatted")
	}
	if IsNotFound(nil) || IsUnauthorized(nil) || IsForbidden(nil) {
		t.Error("nil error matched a reason")
	}
}
