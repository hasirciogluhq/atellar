// Package transport selects how api.Interface travels between processes.
// Every transport carries the same API, so callers pick one by name and the
// rest of the code base never depends on the choice.
package transport

import (
	"context"
	"fmt"
	"net"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	grpcclient "github.com/hasirciogluhq/atellar/pkg/transport/grpc/client"
	grpcserver "github.com/hasirciogluhq/atellar/pkg/transport/grpc/server"
	httpclient "github.com/hasirciogluhq/atellar/pkg/transport/http/client"
	httpserver "github.com/hasirciogluhq/atellar/pkg/transport/http/server"
	"github.com/hasirciogluhq/atellar/pkg/transport/tcp"
)

// Kind names a transport.
type Kind string

const (
	HTTP Kind = "http"
	GRPC Kind = "grpc"
	TCP  Kind = "tcp"
)

// Kinds lists the transports in a stable order.
var Kinds = []Kind{HTTP, GRPC, TCP}

// ParseKind validates a transport name. The empty string selects gRPC.
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case "":
		return GRPC, nil
	case HTTP, GRPC, TCP:
		return Kind(s), nil
	}
	return "", fmt.Errorf("unknown transport %q (want http, grpc or tcp)", s)
}

// Client is an api.Interface bound to a connection.
type Client interface {
	api.Interface
	Close() error
}

// NewClient connects to address. For HTTP, address is a base URL
// ("http://host:port"); for gRPC and TCP it is "host:port". token is called
// per request and may be nil.
func NewClient(kind Kind, address string, token func() string) (Client, error) {
	switch kind {
	case HTTP:
		return httpclient.New(address, token), nil
	case GRPC:
		return grpcclient.New(address, token)
	case TCP:
		_, err := tcp.NewClient(address, token)
		return nil, err
	}
	return nil, fmt.Errorf("unknown transport %q", kind)
}

// Server serves api.Interface until its context is done.
type Server interface {
	ListenAndServe(ctx context.Context, addr string) error
	Serve(ctx context.Context, ln net.Listener) error
}

func NewServer(kind Kind, svc api.Interface, auth authn.Authenticator) (Server, error) {
	switch kind {
	case HTTP:
		return httpserver.New(svc, auth), nil
	case GRPC:
		return grpcserver.New(svc, auth), nil
	case TCP:
		return tcp.NewServer(svc, auth)
	}
	return nil, fmt.Errorf("unknown transport %q", kind)
}
