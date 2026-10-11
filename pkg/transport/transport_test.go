package transport_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/transport"
	"github.com/hasirciogluhq/atellar/pkg/transport/tcp"
)

// wireKinds are the implemented transports; every test runs on each.
var wireKinds = []transport.Kind{transport.HTTP, transport.GRPC}

// serve starts f's API server on a loopback port and returns a client
// factory for it. Everything stops when the test ends.
func serve(t testing.TB, kind transport.Kind, f *apiservertest.Fixture) func(token string) transport.Client {
	connect := serveFunc(t, kind, f)
	return func(token string) transport.Client { return connect(func() string { return token }) }
}

// serveFunc is serve with a token callback, as long running agents use.
func serveFunc(t testing.TB, kind transport.Kind, f *apiservertest.Fixture) func(token func() string) transport.Client {
	t.Helper()
	srv, err := transport.NewServer(kind, f.Server, f.Auth)
	if err != nil {
		t.Fatalf("NewServer(%s): %v", kind, err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	address := ln.Addr().String()
	if kind == transport.HTTP {
		address = "http://" + address
	}
	return func(token func() string) transport.Client {
		c, err := transport.NewClient(kind, address, token)
		if err != nil {
			t.Fatalf("NewClient(%s): %v", kind, err)
		}
		t.Cleanup(func() { _ = c.Close() })
		return c
	}
}

func ctxWithTimeout(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestTransportAuthorizationMatrix repeats the in-process authorization
// matrix over the network: transport x auth mode x method x caller. A
// transport passes when every caller gets exactly the result it gets when
// calling the API server directly.
func TestTransportAuthorizationMatrix(t *testing.T) {
	callers := append([]apiservertest.Caller{}, apiservertest.Callers...)
	callers = append(callers, apiservertest.BadToken)

	for _, kind := range wireKinds {
		for _, open := range []bool{false, true} {
			mode := "closed"
			if open {
				mode = "open"
			}
			for _, tc := range apiservertest.Cases() {
				for _, caller := range callers {
					t.Run(string(kind)+"/"+mode+"/"+tc.Name+"/"+string(caller), func(t *testing.T) {
						t.Parallel()
						f := apiservertest.New(t)
						if open {
							f = apiservertest.NewOpen(t)
						}
						client := serve(t, kind, f)(f.Token(caller))
						err := tc.Call(ctxWithTimeout(t), client, f)
						if err := apiservertest.Check(err, tc.Expect(caller, f.Open)); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}

// TestTransportErrorReasons checks that each error reason survives the
// round trip through every transport.
func TestTransportErrorReasons(t *testing.T) {
	cases := []struct {
		name string
		want api.Reason
		call func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error
	}{
		{"not found", api.ReasonNotFound, func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error {
			_, err := c.Containers().Get(ctx, "missing")
			return err
		}},
		{"bad request", api.ReasonBadRequest, func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error {
			_, err := c.Containers().Create(ctx, v1.CreateContainerRequest{})
			return err
		}},
		{"invalid join token", api.ReasonBadRequest, func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error {
			_, err := c.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: "nope"})
			return err
		}},
		{"conflict", api.ReasonConflict, func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error {
			_, err := c.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: f.JoinToken, Name: f.NodeA.Name})
			return err
		}},
		{"unavailable", api.ReasonUnavailable, func(ctx context.Context, c api.Interface, f *apiservertest.Fixture) error {
			for _, n := range []string{f.NodeA.ID, f.NodeB.ID} {
				if _, err := c.Nodes().Evict(ctx, n); err != nil {
					return err
				}
			}
			_, err := c.Containers().Create(ctx, v1.CreateContainerRequest{Image: "docker.io/library/redis:7"})
			return err
		}},
	}
	for _, kind := range wireKinds {
		for _, tc := range cases {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				f := apiservertest.New(t)
				client := serve(t, kind, f)(apiservertest.AdminToken)
				err := tc.call(ctxWithTimeout(t), client, f)
				if err := apiservertest.Check(err, tc.want); err != nil {
					t.Fatal(err)
				}
				var apiErr *api.Error
				if !errors.As(err, &apiErr) || apiErr.Message == "" {
					t.Fatalf("error %v is not an *api.Error with a message", err)
				}
			})
		}
	}
}

func next[T any](t *testing.T, w watch.Interface[T]) watch.Event[T] {
	t.Helper()
	select {
	case ev, ok := <-w.ResultChan():
		if !ok {
			t.Fatalf("watch closed: %v", w.Err())
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watch event")
	}
	panic("unreachable")
}

// TestTransportWatch follows the agent's view of its own node: snapshot,
// bookmark, then live changes made by the admin, then a clean stop.
func TestTransportWatch(t *testing.T) {
	for _, kind := range wireKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			f := apiservertest.New(t)
			connect := serve(t, kind, f)
			agent, admin := connect(f.KeyA), connect(apiservertest.AdminToken)
			ctx := ctxWithTimeout(t)

			w, err := agent.Containers().Watch(ctx, v1.ContainerListOptions{NodeID: f.NodeA.ID})
			if err != nil {
				t.Fatal(err)
			}
			if ev := next(t, w); ev.Type != watch.Added || ev.Object.ID != f.Container.ID {
				t.Fatalf("first event = %s %s, want ADDED %s", ev.Type, ev.Object.ID, f.Container.ID)
			}
			if ev := next(t, w); ev.Type != watch.Bookmark {
				t.Fatalf("second event = %s, want BOOKMARK", ev.Type)
			}

			if _, err := admin.Containers().Delete(ctx, f.Container.ID); err != nil {
				t.Fatal(err)
			}
			ev := next(t, w)
			if ev.Type != watch.Modified || ev.Object.Status != v1.ContainerStatusRemoved {
				t.Fatalf("live event = %s %s, want MODIFIED removed", ev.Type, ev.Object.Status)
			}

			// A change on another node must not reach this watch; the next
			// event must be the one made on node A afterwards.
			// Shrink node A so the scheduler must pick node B.
			if _, err := f.Store.UpdateNodeHardware(ctx, f.NodeA.ID, v1.NodeHardware{CpuCores: 1, MemoryTotalMiB: 128}); err != nil {
				t.Fatal(err)
			}
			mem := int32(1024)
			created, err := admin.Containers().Create(ctx, v1.CreateContainerRequest{Image: "docker.io/library/redis:7", MemoryLimitMiB: &mem})
			if err != nil {
				t.Fatal(err)
			}
			if created.NodeID != f.NodeB.ID {
				t.Fatalf("scheduled on %s, want node B", created.NodeID)
			}
			if _, err := agent.Containers().ReportRuntime(ctx, f.Container.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusTerminated}); err != nil {
				t.Fatal(err)
			}
			if ev := next(t, w); ev.Object.ID != f.Container.ID || ev.Object.Status != v1.ContainerStatusTerminated {
				t.Fatalf("got %s %s %s, want container %s terminated", ev.Type, ev.Object.ID, ev.Object.Status, f.Container.ID)
			}

			w.Stop()
			for range w.ResultChan() {
			}
			if err := w.Err(); err != nil {
				t.Fatalf("Err after Stop = %v, want nil", err)
			}
		})
	}
}

// TestTransportNodeWatch checks node events reach a watcher over the wire.
func TestTransportNodeWatch(t *testing.T) {
	for _, kind := range wireKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			f := apiservertest.New(t)
			connect := serve(t, kind, f)
			ctx := ctxWithTimeout(t)

			w, err := connect(f.KeyA).Nodes().Watch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Stop()
			for i := 0; i < 2; i++ {
				if ev := next(t, w); ev.Type != watch.Added {
					t.Fatalf("snapshot event %d = %s", i, ev.Type)
				}
			}
			if ev := next(t, w); ev.Type != watch.Bookmark {
				t.Fatalf("got %s, want BOOKMARK", ev.Type)
			}
			if _, err := connect(apiservertest.AdminToken).Nodes().Evict(ctx, f.NodeB.Name); err != nil {
				t.Fatal(err)
			}
			ev := next(t, w)
			if ev.Type != watch.Modified || ev.Object.ID != f.NodeB.ID || ev.Object.Status != v1.NodeStatusEvicted {
				t.Fatalf("got %s %s %s, want MODIFIED %s evicted", ev.Type, ev.Object.ID, ev.Object.Status, f.NodeB.ID)
			}
		})
	}
}

// TestTransportKeyRotation checks the client picks up a renewed key on the
// next call and the old key stops working.
func TestTransportKeyRotation(t *testing.T) {
	for _, kind := range wireKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			f := apiservertest.New(t)
			connect := serve(t, kind, f)
			ctx := ctxWithTimeout(t)

			var mu sync.Mutex
			key := f.KeyA
			agent := serveFunc(t, kind, f)(func() string { mu.Lock(); defer mu.Unlock(); return key })
			renewed, err := agent.Nodes().SelfRenewAPIKey(ctx)
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			old := key
			key = renewed.NodeAPIKey
			mu.Unlock()
			if err := agent.Nodes().SelfHeartbeat(ctx); err != nil {
				t.Fatalf("heartbeat with renewed key: %v", err)
			}
			err = connect(old).Nodes().SelfHeartbeat(ctx)
			if err := apiservertest.Check(err, api.ReasonUnauthorized); err != nil {
				t.Fatalf("old key: %v", err)
			}
		})
	}
}

// TestParseKind covers every accepted and rejected transport name.
func TestParseKind(t *testing.T) {
	cases := map[string]struct {
		want transport.Kind
		ok   bool
	}{
		"":     {transport.GRPC, true},
		"http": {transport.HTTP, true},
		"grpc": {transport.GRPC, true},
		"tcp":  {transport.TCP, true},
		"HTTP": {"", false},
		"quic": {"", false},
	}
	for in, tc := range cases {
		got, err := transport.ParseKind(in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("ParseKind(%q) = %q, %v; want %q ok=%v", in, got, err, tc.want, tc.ok)
		}
	}
}

// TestTCPNotImplemented checks the reserved transport fails clearly.
func TestTCPNotImplemented(t *testing.T) {
	f := apiservertest.New(t)
	if _, err := transport.NewServer(transport.TCP, f.Server, f.Auth); !errors.Is(err, tcp.ErrNotImplemented) {
		t.Errorf("NewServer(tcp) error = %v", err)
	}
	if _, err := transport.NewClient(transport.TCP, "127.0.0.1:1", nil); !errors.Is(err, tcp.ErrNotImplemented) {
		t.Errorf("NewClient(tcp) error = %v", err)
	}
	if _, err := transport.NewClient("smoke", "x", nil); err == nil {
		t.Error("NewClient(unknown) succeeded")
	}
}
