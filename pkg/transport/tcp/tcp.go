// Package tcp reserves a raw TCP transport with a length-prefixed binary
// framing. It is not implemented yet; HTTP and gRPC cover every use today.
// The constructors exist so the transport selection code and configuration
// already accept the name and fail with a clear error.
package tcp

import (
	"context"
	"errors"
	"net"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
)

// ErrNotImplemented is returned by every constructor in this package.
var ErrNotImplemented = errors.New("tcp transport is not implemented yet; use http or grpc")

// Server will serve api.Interface over raw TCP.
type Server struct{}

func NewServer(api.Interface, authn.Authenticator) (*Server, error) { return nil, ErrNotImplemented }

func (*Server) ListenAndServe(context.Context, string) error { return ErrNotImplemented }
func (*Server) Serve(context.Context, net.Listener) error    { return ErrNotImplemented }

// NewClient will return an api.Interface speaking raw TCP.
func NewClient(string, func() string) (api.Interface, error) { return nil, ErrNotImplemented }
