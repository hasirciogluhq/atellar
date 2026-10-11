// Package wire is the gRPC mapping shared by the gRPC server and client:
// a JSON codec, the hand-written service description, request envelopes
// and status code mapping. No .proto files or generated code are involved;
// messages are the pkg/apis types encoded as JSON.
package wire

import (
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"

	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

// CodecName is the gRPC content subtype ("application/grpc+json").
const CodecName = "json"

type codec struct{}

func (codec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (codec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (codec) Name() string                       { return CodecName }

func init() {
	encoding.RegisterCodec(codec{})
}

// ServiceName is the fully qualified gRPC service.
const ServiceName = "atellar.v1.Atellar"

// Method names. Full method is "/" + ServiceName + "/" + name.
const (
	NodesList           = "NodesList"
	NodesGet            = "NodesGet"
	NodesWatch          = "NodesWatch"
	NodesEvict          = "NodesEvict"
	NodesListOverlayIPs = "NodesListOverlayIPs"
	NodesRegister       = "NodesRegister"
	NodesSelfHeartbeat  = "NodesSelfHeartbeat"
	NodesSelfHardware   = "NodesSelfHardware"
	NodesSelfRenewKey   = "NodesSelfRenewAPIKey"

	ContainersList          = "ContainersList"
	ContainersGet           = "ContainersGet"
	ContainersWatch         = "ContainersWatch"
	ContainersCreate        = "ContainersCreate"
	ContainersDelete        = "ContainersDelete"
	ContainersListEvents    = "ContainersListEvents"
	ContainersReportRuntime = "ContainersReportRuntime"
	ContainersAllocateIP    = "ContainersAllocateOverlayIP"

	JoinTokensList   = "JoinTokensList"
	JoinTokensCreate = "JoinTokensCreate"
)

func FullMethod(name string) string { return "/" + ServiceName + "/" + name }

// MetadataAuthorization carries "Bearer <token>".
const MetadataAuthorization = "authorization"

// Empty is the request or response of calls without a payload.
type Empty struct{}

// Ref addresses one object by ID or name.
type Ref struct {
	Ref string `json:"ref"`
}

type OverlayIPsRequest struct {
	Ref      string `json:"ref"`
	FreeOnly bool   `json:"free_only,omitempty"`
}

type RuntimeReportRequest struct {
	ID     string                    `json:"id"`
	Report v1.ContainerRuntimeReport `json:"report"`
}

// Event is one message of a watch stream.
type Event struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object,omitempty"`
}

var reasonCode = map[api.Reason]codes.Code{
	api.ReasonBadRequest:   codes.InvalidArgument,
	api.ReasonUnauthorized: codes.Unauthenticated,
	api.ReasonForbidden:    codes.PermissionDenied,
	api.ReasonNotFound:     codes.NotFound,
	api.ReasonConflict:     codes.AlreadyExists,
	api.ReasonUnavailable:  codes.Unavailable,
	api.ReasonInternal:     codes.Internal,
}

// ToStatus converts an error from api.Interface into a gRPC status error.
func ToStatus(err error) error {
	if err == nil {
		return nil
	}
	reason := api.ReasonOf(err)
	code, ok := reasonCode[reason]
	if !ok {
		code = codes.Internal
	}
	return status.Error(code, err.Error())
}

// FromStatus converts a gRPC error back into an api.Error.
func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return api.Unavailable("%v", err)
	}
	for reason, code := range reasonCode {
		if code == st.Code() {
			return &api.Error{Reason: reason, Message: st.Message()}
		}
	}
	switch st.Code() {
	case codes.Canceled, codes.DeadlineExceeded:
		return &api.Error{Reason: api.ReasonUnavailable, Message: st.Message()}
	}
	return &api.Error{Reason: api.ReasonInternal, Message: st.Message()}
}

// Unmarshal decodes a watch event object.
func Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
