package wire

import (
	"net/http"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/api"
)

func TestStatusRoundTrip(t *testing.T) {
	reasons := []api.Reason{
		api.ReasonBadRequest, api.ReasonUnauthorized, api.ReasonForbidden, api.ReasonNotFound,
		api.ReasonConflict, api.ReasonUnavailable, api.ReasonInternal,
	}
	seen := map[int]bool{}
	for _, reason := range reasons {
		status := Status(reason)
		if seen[status] {
			t.Errorf("status %d used by more than one reason", status)
		}
		seen[status] = true
		if got := Reason(status); got != reason {
			t.Errorf("Reason(Status(%s)) = %s", reason, got)
		}
	}
	if got := Status("Unknown"); got != http.StatusInternalServerError {
		t.Errorf("Status(unknown) = %d", got)
	}
	if got := Reason(http.StatusTeapot); got != api.ReasonInternal {
		t.Errorf("Reason(418) = %s", got)
	}
}
