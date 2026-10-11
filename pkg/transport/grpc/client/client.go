// Package client is the gRPC implementation of api.Interface for callers.
// It uses the JSON codec from pkg/transport/grpc/wire, so no generated code
// is required on either side.
package client

import (
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/transport/grpc/wire"
)

const (
	defaultTimeout = 15 * time.Second
	watchBuffer    = 64
)

// Client talks to an Atellar API server over one multiplexed gRPC connection.
type Client struct {
	conn  *grpc.ClientConn
	token func() string
}

var _ api.Interface = (*Client)(nil)

// New dials target ("host:port") lazily. token may be nil; it is called per
// call so rotated keys take effect immediately.
func New(target string, token func() string) (*Client, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(wire.CodecName)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second, Timeout: 10 * time.Second, PermitWithoutStream: true}),
	)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, token: token}, nil
}

// Close releases the connection.
func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) Nodes() api.NodeInterface           { return nodes{c} }
func (c *Client) Containers() api.ContainerInterface { return containers{c} }
func (c *Client) JoinTokens() api.JoinTokenInterface { return joinTokens{c} }

func (c *Client) outgoing(ctx context.Context) context.Context {
	if c.token == nil {
		return ctx
	}
	if tok := c.token(); tok != "" {
		return metadata.AppendToOutgoingContext(ctx, wire.MetadataAuthorization, authn.Bearer(tok))
	}
	return ctx
}

func invoke[Resp any](ctx context.Context, c *Client, method string, req any) (Resp, error) {
	var out Resp
	ctx = c.outgoing(ctx)
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}
	err := c.conn.Invoke(ctx, wire.FullMethod(method), req, &out)
	return out, wire.FromStatus(err)
}

func invokePtr[Resp any](ctx context.Context, c *Client, method string, req any) (*Resp, error) {
	out, err := invoke[Resp](ctx, c, method, req)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

var streamDesc = grpc.StreamDesc{ServerStreams: true}

// watchMethod opens a server stream and decodes it on a goroutine.
func watchMethod[T any](ctx context.Context, c *Client, method string, req any) (watch.Interface[T], error) {
	stream := watch.NewStream[T](ctx, watchBuffer)
	cs, err := c.conn.NewStream(c.outgoing(stream.Context()), &streamDesc, wire.FullMethod(method))
	if err != nil {
		stream.Stop()
		return nil, wire.FromStatus(err)
	}
	if err := cs.SendMsg(req); err != nil {
		stream.Stop()
		return nil, wire.FromStatus(err)
	}
	if err := cs.CloseSend(); err != nil {
		stream.Stop()
		return nil, wire.FromStatus(err)
	}
	// The server authenticates before the first message. Waiting for the
	// header surfaces auth and validation errors from Watch itself.
	md, err := cs.Header()
	if err == nil && md == nil {
		// Trailers-only response: the call failed before any event.
		var frame wire.Event
		if err = cs.RecvMsg(&frame); errors.Is(err, io.EOF) {
			err = api.Unavailable("watch %s: stream ended", method)
		}
	}
	if err != nil {
		stream.Stop()
		return nil, wire.FromStatus(err)
	}

	go func() {
		for {
			var frame wire.Event
			err := cs.RecvMsg(&frame)
			if err != nil {
				switch {
				case stream.Context().Err() != nil:
					err = nil // stopped by the consumer
				case errors.Is(err, io.EOF):
					err = api.Unavailable("watch %s: stream ended", method)
				default:
					err = wire.FromStatus(err)
				}
				stream.Close(err)
				return
			}
			ev := watch.Event[T]{Type: watch.EventType(frame.Type)}
			if len(frame.Object) > 0 {
				if err := wire.Unmarshal(frame.Object, &ev.Object); err != nil {
					stream.Stop()
					stream.Close(api.Internal("watch %s: bad object: %v", method, err))
					return
				}
			}
			if !stream.Send(ev) {
				stream.Close(nil)
				return
			}
		}
	}()
	return stream, nil
}

type nodes struct{ c *Client }

func (n nodes) List(ctx context.Context) ([]v1.Node, error) {
	return invoke[[]v1.Node](ctx, n.c, wire.NodesList, wire.Empty{})
}

func (n nodes) Get(ctx context.Context, ref string) (*v1.Node, error) {
	return invokePtr[v1.Node](ctx, n.c, wire.NodesGet, wire.Ref{Ref: ref})
}

func (n nodes) Watch(ctx context.Context) (watch.Interface[v1.Node], error) {
	return watchMethod[v1.Node](ctx, n.c, wire.NodesWatch, wire.Empty{})
}

func (n nodes) Evict(ctx context.Context, ref string) (*v1.Node, error) {
	return invokePtr[v1.Node](ctx, n.c, wire.NodesEvict, wire.Ref{Ref: ref})
}

func (n nodes) ListOverlayIPs(ctx context.Context, ref string, freeOnly bool) ([]v1.OverlayIP, error) {
	return invoke[[]v1.OverlayIP](ctx, n.c, wire.NodesListOverlayIPs, wire.OverlayIPsRequest{Ref: ref, FreeOnly: freeOnly})
}

func (n nodes) Register(ctx context.Context, req v1.RegisterNodeRequest) (*v1.RegisterNodeResponse, error) {
	return invokePtr[v1.RegisterNodeResponse](ctx, n.c, wire.NodesRegister, req)
}

func (n nodes) SelfHeartbeat(ctx context.Context) error {
	_, err := invoke[wire.Empty](ctx, n.c, wire.NodesSelfHeartbeat, wire.Empty{})
	return err
}

func (n nodes) SelfReportHardware(ctx context.Context, hw v1.NodeHardware) error {
	_, err := invoke[wire.Empty](ctx, n.c, wire.NodesSelfHardware, hw)
	return err
}

func (n nodes) SelfRenewAPIKey(ctx context.Context) (*v1.NodeAPIKey, error) {
	return invokePtr[v1.NodeAPIKey](ctx, n.c, wire.NodesSelfRenewKey, wire.Empty{})
}

type containers struct{ c *Client }

func (cs containers) List(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error) {
	return invoke[[]v1.Container](ctx, cs.c, wire.ContainersList, opts)
}

func (cs containers) Get(ctx context.Context, id string) (*v1.Container, error) {
	return invokePtr[v1.Container](ctx, cs.c, wire.ContainersGet, wire.Ref{Ref: id})
}

func (cs containers) Watch(ctx context.Context, opts v1.ContainerListOptions) (watch.Interface[v1.Container], error) {
	return watchMethod[v1.Container](ctx, cs.c, wire.ContainersWatch, opts)
}

func (cs containers) Create(ctx context.Context, req v1.CreateContainerRequest) (*v1.Container, error) {
	return invokePtr[v1.Container](ctx, cs.c, wire.ContainersCreate, req)
}

func (cs containers) Delete(ctx context.Context, id string) (*v1.Container, error) {
	return invokePtr[v1.Container](ctx, cs.c, wire.ContainersDelete, wire.Ref{Ref: id})
}

func (cs containers) ListEvents(ctx context.Context, id string) ([]v1.ContainerEvent, error) {
	return invoke[[]v1.ContainerEvent](ctx, cs.c, wire.ContainersListEvents, wire.Ref{Ref: id})
}

func (cs containers) ReportRuntime(ctx context.Context, id string, report v1.ContainerRuntimeReport) (*v1.Container, error) {
	return invokePtr[v1.Container](ctx, cs.c, wire.ContainersReportRuntime, wire.RuntimeReportRequest{ID: id, Report: report})
}

func (cs containers) AllocateOverlayIP(ctx context.Context, id string) (*v1.Container, error) {
	return invokePtr[v1.Container](ctx, cs.c, wire.ContainersAllocateIP, wire.Ref{Ref: id})
}

type joinTokens struct{ c *Client }

func (j joinTokens) List(ctx context.Context) ([]v1.JoinToken, error) {
	return invoke[[]v1.JoinToken](ctx, j.c, wire.JoinTokensList, wire.Empty{})
}

func (j joinTokens) Create(ctx context.Context, req v1.CreateJoinTokenRequest) (*v1.JoinToken, error) {
	return invokePtr[v1.JoinToken](ctx, j.c, wire.JoinTokensCreate, req)
}
