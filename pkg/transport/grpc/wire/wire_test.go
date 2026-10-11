package wire

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hasirciogluhq/atellar/pkg/api"
)

func TestStatusRoundTrip(t *testing.T) {
	reasons := []api.Reason{
		api.ReasonBadRequest, api.ReasonUnauthorized, api.ReasonForbidden, api.ReasonNotFound,
		api.ReasonConflict, api.ReasonUnavailable, api.ReasonInternal,
	}
	for _, reason := range reasons {
		in := &api.Error{Reason: reason, Message: "boom"}
		out := FromStatus(ToStatus(in))
		var apiErr *api.Error
		if !errors.As(out, &apiErr) || apiErr.Reason != reason || apiErr.Message != "boom" {
			t.Errorf("round trip of %s = %#v", reason, out)
		}
	}
}

func TestStatusEdges(t *testing.T) {
	if ToStatus(nil) != nil || FromStatus(nil) != nil {
		t.Fatal("nil must map to nil")
	}
	if code := status.Code(ToStatus(errors.New("plain"))); code != codes.Internal {
		t.Errorf("foreign error code = %s, want Internal", code)
	}
	cases := map[codes.Code]api.Reason{
		codes.Canceled:         api.ReasonUnavailable,
		codes.DeadlineExceeded: api.ReasonUnavailable,
		codes.Unimplemented:    api.ReasonInternal,
	}
	for code, want := range cases {
		if got := api.ReasonOf(FromStatus(status.Error(code, "x"))); got != want {
			t.Errorf("FromStatus(%s) reason = %s, want %s", code, got, want)
		}
	}
	if got := api.ReasonOf(FromStatus(context.Canceled)); got != api.ReasonUnavailable {
		t.Errorf("FromStatus(non-status) reason = %s", got)
	}
}

func TestCodec(t *testing.T) {
	var c codec
	data, err := c.Marshal(Ref{Ref: "node-a"})
	if err != nil {
		t.Fatal(err)
	}
	var out Ref
	if err := c.Unmarshal(data, &out); err != nil || out.Ref != "node-a" {
		t.Fatalf("Unmarshal = %+v, %v", out, err)
	}
	if c.Name() != CodecName {
		t.Fatalf("Name = %s", c.Name())
	}
}
