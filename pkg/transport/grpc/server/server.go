// Package server exposes an api.Interface over gRPC with the JSON codec.
package server

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/transport/grpc/wire"
)

// Server wraps a grpc.Server serving the Atellar service.
type Server struct {
	grpc *grpc.Server
}

func New(svc api.Interface, auth authn.Authenticator) *Server {
	g := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unaryAuth(auth)),
		grpc.ChainStreamInterceptor(streamAuth(auth)),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second, Timeout: 10 * time.Second}),
	)
	g.RegisterService(serviceDesc(svc), nil)
	return &Server{grpc: g}
}

// ListenAndServe serves until ctx is done, then stops gracefully.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		stopped := make(chan struct{})
		go func() { s.grpc.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			s.grpc.Stop()
		}
	}()
	return s.grpc.Serve(ln)
}

func authenticate(ctx context.Context, auth authn.Authenticator) (context.Context, error) {
	var header string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(wire.MetadataAuthorization); len(values) > 0 {
			header = values[0]
		}
	}
	tok, err := authn.ParseBearer(header)
	if err != nil {
		return nil, wire.ToStatus(api.Unauthorized("%v", err))
	}
	principal, err := auth.Authenticate(ctx, tok)
	if err != nil {
		return nil, wire.ToStatus(api.Unauthorized("%v", err))
	}
	if principal != nil {
		ctx = authn.WithPrincipal(ctx, principal)
	}
	return ctx, nil
}

func unaryAuth(auth authn.Authenticator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := authenticate(ctx, auth)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

type authedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s authedStream) Context() context.Context { return s.ctx }

func streamAuth(auth authn.Authenticator) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := authenticate(ss.Context(), auth)
		if err != nil {
			return err
		}
		return handler(srv, authedStream{ServerStream: ss, ctx: ctx})
	}
}

// unary adapts a typed function into a grpc.MethodDesc.
func unary[Req, Resp any](name string, fn func(context.Context, Req) (Resp, error)) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(_ any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			var req Req
			if err := dec(&req); err != nil {
				return nil, wire.ToStatus(api.BadRequest("decode request: %v", err))
			}
			call := func(ctx context.Context, r any) (any, error) {
				resp, err := fn(ctx, *r.(*Req))
				if err != nil {
					return nil, wire.ToStatus(err)
				}
				return resp, nil
			}
			if interceptor == nil {
				return call(ctx, &req)
			}
			return interceptor(ctx, &req, &grpc.UnaryServerInfo{FullMethod: wire.FullMethod(name)}, call)
		},
	}
}

// watchStream adapts a typed watch into a server-streaming grpc.StreamDesc.
func watchStream[Req, T any](name string, open func(context.Context, Req) (watch.Interface[T], error)) grpc.StreamDesc {
	return grpc.StreamDesc{
		StreamName:    name,
		ServerStreams: true,
		Handler: func(_ any, stream grpc.ServerStream) error {
			var req Req
			if err := stream.RecvMsg(&req); err != nil {
				return wire.ToStatus(api.BadRequest("decode request: %v", err))
			}
			wi, err := open(stream.Context(), req)
			if err != nil {
				return wire.ToStatus(err)
			}
			defer wi.Stop()
			for {
				select {
				case <-stream.Context().Done():
					return nil
				case ev, ok := <-wi.ResultChan():
					if !ok {
						return wire.ToStatus(wi.Err())
					}
					obj, err := json.Marshal(ev.Object)
					if err != nil {
						return wire.ToStatus(api.Internal("encode event: %v", err))
					}
					if err := stream.SendMsg(&wire.Event{Type: string(ev.Type), Object: obj}); err != nil {
						return err
					}
				}
			}
		},
	}
}

func serviceDesc(svc api.Interface) *grpc.ServiceDesc {
	n, c, j := svc.Nodes(), svc.Containers(), svc.JoinTokens()
	empty := wire.Empty{}

	return &grpc.ServiceDesc{
		ServiceName: wire.ServiceName,
		HandlerType: (*any)(nil),
		Metadata:    "atellar/v1",
		Methods: []grpc.MethodDesc{
			unary(wire.NodesList, func(ctx context.Context, _ wire.Empty) ([]v1.Node, error) { return n.List(ctx) }),
			unary(wire.NodesGet, func(ctx context.Context, r wire.Ref) (*v1.Node, error) { return n.Get(ctx, r.Ref) }),
			unary(wire.NodesEvict, func(ctx context.Context, r wire.Ref) (*v1.Node, error) { return n.Evict(ctx, r.Ref) }),
			unary(wire.NodesListOverlayIPs, func(ctx context.Context, r wire.OverlayIPsRequest) ([]v1.OverlayIP, error) {
				return n.ListOverlayIPs(ctx, r.Ref, r.FreeOnly)
			}),
			unary(wire.NodesRegister, n.Register),
			unary(wire.NodesSelfHeartbeat, func(ctx context.Context, _ wire.Empty) (wire.Empty, error) {
				return empty, n.SelfHeartbeat(ctx)
			}),
			unary(wire.NodesSelfHardware, func(ctx context.Context, hw v1.NodeHardware) (wire.Empty, error) {
				return empty, n.SelfReportHardware(ctx, hw)
			}),
			unary(wire.NodesSelfRenewKey, func(ctx context.Context, _ wire.Empty) (*v1.NodeAPIKey, error) {
				return n.SelfRenewAPIKey(ctx)
			}),

			unary(wire.ContainersList, c.List),
			unary(wire.ContainersGet, func(ctx context.Context, r wire.Ref) (*v1.Container, error) { return c.Get(ctx, r.Ref) }),
			unary(wire.ContainersCreate, c.Create),
			unary(wire.ContainersDelete, func(ctx context.Context, r wire.Ref) (*v1.Container, error) { return c.Delete(ctx, r.Ref) }),
			unary(wire.ContainersListEvents, func(ctx context.Context, r wire.Ref) ([]v1.ContainerEvent, error) {
				return c.ListEvents(ctx, r.Ref)
			}),
			unary(wire.ContainersReportRuntime, func(ctx context.Context, r wire.RuntimeReportRequest) (*v1.Container, error) {
				return c.ReportRuntime(ctx, r.ID, r.Report)
			}),
			unary(wire.ContainersAllocateIP, func(ctx context.Context, r wire.Ref) (*v1.Container, error) {
				return c.AllocateOverlayIP(ctx, r.Ref)
			}),

			unary(wire.JoinTokensList, func(ctx context.Context, _ wire.Empty) ([]v1.JoinToken, error) { return j.List(ctx) }),
			unary(wire.JoinTokensCreate, j.Create),
		},
		Streams: []grpc.StreamDesc{
			watchStream(wire.NodesWatch, func(ctx context.Context, _ wire.Empty) (watch.Interface[v1.Node], error) {
				return n.Watch(ctx)
			}),
			watchStream(wire.ContainersWatch, c.Watch),
		},
	}
}
