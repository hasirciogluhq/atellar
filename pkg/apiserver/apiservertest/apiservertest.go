// Package apiservertest builds a complete API server on the in-memory store
// and describes the authorization matrix every transport must reproduce.
// It is used by tests only.
package apiservertest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver"
	"github.com/hasirciogluhq/atellar/pkg/auth"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/memory"
)

// AdminToken is the admin bearer token of a closed fixture.
const AdminToken = "test-admin-token"

// Caller is who makes a request in the matrix.
type Caller string

const (
	Anonymous Caller = "anonymous"
	Admin     Caller = "admin"
	// NodeSelf is node A, which owns Fixture.Container.
	NodeSelf Caller = "node-self"
	// OtherNode is node B.
	OtherNode Caller = "other-node"
	// BadToken presents a token nobody issued. Only transports see it.
	BadToken Caller = "bad-token"
)

// Callers are the principals every in-process case is checked against.
var Callers = []Caller{Anonymous, Admin, NodeSelf, OtherNode}

// Fixture is a running API server with two registered nodes and one
// container scheduled on node A.
type Fixture struct {
	Store  *memory.Store
	Server *apiserver.Server
	Auth   *auth.TokenAuthenticator
	// Open is true when the fixture has no admin token.
	Open bool

	NodeA, NodeB *v1.Node
	KeyA, KeyB   string
	Container    *v1.Container
	// JoinToken is an unused single-use token.
	JoinToken string
}

// New builds a closed fixture (admin token required).
func New(t testing.TB) *Fixture { return build(t, AdminToken) }

// NewOpen builds a fixture without an admin token: anonymous callers are
// treated as admin.
func NewOpen(t testing.TB) *Fixture { return build(t, "") }

func build(t testing.TB, adminToken string) *Fixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	srv, err := apiserver.New(apiserver.Config{Store: store, ClusterCIDR: "10.0.0.0/16", NodeSubnetPrefixLen: 24})
	if err != nil {
		t.Fatalf("apiserver.New: %v", err)
	}
	f := &Fixture{
		Store:  store,
		Server: srv,
		Auth:   auth.NewTokenAuthenticator(adminToken, store.GetNodeByAPIKeyHash),
		Open:   adminToken == "",
	}

	admin := f.Context(t, Admin)
	f.NodeA, f.KeyA = f.register(t, admin, "node-a", "192.168.0.1")
	f.NodeB, f.KeyB = f.register(t, admin, "node-b", "192.168.0.2")
	f.JoinToken = f.NewJoinToken(t)

	created, err := store.CreateContainer(ctx, storage.CreateContainerParams{
		NodeID:                 f.NodeA.ID,
		CreateContainerRequest: v1.CreateContainerRequest{Image: "docker.io/library/nginx:1.27"},
	})
	if err != nil {
		t.Fatalf("create container: %v", err)
	}
	if f.Container, err = store.ScheduleContainer(ctx, created.ID); err != nil {
		t.Fatalf("schedule container: %v", err)
	}
	return f
}

func (f *Fixture) register(t testing.TB, admin context.Context, name, privateIP string) (*v1.Node, string) {
	t.Helper()
	resp, err := f.Server.Nodes().Register(context.Background(), v1.RegisterNodeRequest{
		JoinToken: f.NewJoinToken(t), Name: name, PrivateIP: privateIP,
	})
	if err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
	node, err := f.Store.UpdateNodeHardware(admin, resp.Node.ID, v1.NodeHardware{CpuCores: 4, MemoryTotalMiB: 8192, DiskTotalGiB: 100})
	if err != nil {
		t.Fatalf("hardware %s: %v", name, err)
	}
	return node, resp.NodeAPIKey
}

// NewJoinToken mints a fresh single-use token as admin.
func (f *Fixture) NewJoinToken(t testing.TB) string {
	t.Helper()
	jt, err := f.Server.JoinTokens().Create(f.Context(t, Admin), v1.CreateJoinTokenRequest{})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}
	return jt.Token
}

// Token returns the bearer token a caller presents ("" for anonymous).
func (f *Fixture) Token(c Caller) string {
	switch c {
	case Admin:
		if f.Open {
			return ""
		}
		return AdminToken
	case NodeSelf:
		return f.KeyA
	case OtherNode:
		return f.KeyB
	case BadToken:
		return "not-a-real-token"
	}
	return ""
}

// Context authenticates the caller's token the way a transport would.
func (f *Fixture) Context(t testing.TB, c Caller) context.Context {
	t.Helper()
	ctx := context.Background()
	principal, err := f.Auth.Authenticate(ctx, f.Token(c))
	if err != nil {
		t.Fatalf("authenticate %s: %v", c, err)
	}
	if principal != nil {
		ctx = authn.WithPrincipal(ctx, principal)
	}
	return ctx
}

// Case is one API method called with fixed arguments.
type Case struct {
	Name string
	Call func(ctx context.Context, c api.Interface, f *Fixture) error
	// Deny maps callers to the expected refusal. Callers not listed must
	// succeed.
	Deny map[Caller]api.Reason
}

// Expect returns the reason a caller must get ("" for success).
func (c Case) Expect(caller Caller, open bool) api.Reason {
	switch {
	case caller == BadToken:
		return api.ReasonUnauthorized
	case caller == Anonymous && open:
		caller = Admin
	}
	return c.Deny[caller]
}

const (
	unauth    = api.ReasonUnauthorized
	forbidden = api.ReasonForbidden
)

var (
	adminOnly     = map[Caller]api.Reason{Anonymous: unauth, NodeSelf: forbidden, OtherNode: forbidden}
	authenticated = map[Caller]api.Reason{Anonymous: unauth}
	nodesOnly     = map[Caller]api.Reason{Anonymous: unauth, Admin: forbidden}
	ownerOnly     = map[Caller]api.Reason{Anonymous: unauth, Admin: forbidden, OtherNode: forbidden}
	adminOrOwner  = map[Caller]api.Reason{Anonymous: unauth, OtherNode: forbidden}
)

// Cases covers every method of api.Interface.
func Cases() []Case {
	return []Case{
		{"Nodes.List", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().List(ctx)
			return err
		}, authenticated},
		{"Nodes.Get", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().Get(ctx, f.NodeA.Name)
			return err
		}, authenticated},
		{"Nodes.Watch", func(ctx context.Context, c api.Interface, f *Fixture) error {
			w, err := c.Nodes().Watch(ctx)
			if err != nil {
				return err
			}
			return ReadSnapshot(w, 2)
		}, authenticated},
		{"Nodes.Evict", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().Evict(ctx, f.NodeB.ID)
			return err
		}, adminOnly},
		{"Nodes.ListOverlayIPs", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().ListOverlayIPs(ctx, f.NodeA.ID, true)
			return err
		}, adminOnly},
		{"Nodes.Register", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: f.JoinToken, Name: "node-c", PrivateIP: "192.168.0.3"})
			return err
		}, nil},
		{"Nodes.SelfHeartbeat", func(ctx context.Context, c api.Interface, f *Fixture) error {
			return c.Nodes().SelfHeartbeat(ctx)
		}, nodesOnly},
		{"Nodes.SelfReportHardware", func(ctx context.Context, c api.Interface, f *Fixture) error {
			return c.Nodes().SelfReportHardware(ctx, v1.NodeHardware{CpuCores: 8, MemoryTotalMiB: 16384})
		}, nodesOnly},
		{"Nodes.SelfRenewAPIKey", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Nodes().SelfRenewAPIKey(ctx)
			return err
		}, nodesOnly},
		{"Containers.List(all)", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().List(ctx, v1.ContainerListOptions{})
			return err
		}, adminOnly},
		{"Containers.List(node A)", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().List(ctx, v1.ContainerListOptions{NodeID: f.NodeA.ID, ActiveOnly: true})
			return err
		}, adminOrOwner},
		{"Containers.Get", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().Get(ctx, f.Container.ID)
			return err
		}, adminOnly},
		{"Containers.Watch(node A)", func(ctx context.Context, c api.Interface, f *Fixture) error {
			w, err := c.Containers().Watch(ctx, v1.ContainerListOptions{NodeID: f.NodeA.ID})
			if err != nil {
				return err
			}
			return ReadSnapshot(w, 1)
		}, adminOrOwner},
		{"Containers.Watch(all)", func(ctx context.Context, c api.Interface, f *Fixture) error {
			w, err := c.Containers().Watch(ctx, v1.ContainerListOptions{})
			if err != nil {
				return err
			}
			return ReadSnapshot(w, 1)
		}, adminOnly},
		{"Containers.Create", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().Create(ctx, v1.CreateContainerRequest{Image: "docker.io/library/redis:7"})
			return err
		}, adminOnly},
		{"Containers.Delete", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().Delete(ctx, f.Container.ID)
			return err
		}, adminOnly},
		{"Containers.ListEvents", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().ListEvents(ctx, f.Container.ID)
			return err
		}, adminOnly},
		{"Containers.ReportRuntime", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().ReportRuntime(ctx, f.Container.ID, v1.ContainerRuntimeReport{Status: v1.ContainerStatusPulling})
			return err
		}, ownerOnly},
		{"Containers.AllocateOverlayIP", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.Containers().AllocateOverlayIP(ctx, f.Container.ID)
			return err
		}, ownerOnly},
		{"JoinTokens.List", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.JoinTokens().List(ctx)
			return err
		}, adminOnly},
		{"JoinTokens.Create", func(ctx context.Context, c api.Interface, f *Fixture) error {
			_, err := c.JoinTokens().Create(ctx, v1.CreateJoinTokenRequest{})
			return err
		}, adminOnly},
	}
}

// ReadSnapshot reads a watch up to its BOOKMARK, checks it saw exactly want
// ADDED events, and stops the watch.
func ReadSnapshot[T any](w watch.Interface[T], want int) error {
	defer w.Stop()
	added := 0
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-w.ResultChan():
			if !ok {
				if err := w.Err(); err != nil {
					return err
				}
				return errors.New("watch closed before bookmark")
			}
			switch ev.Type {
			case watch.Added:
				added++
			case watch.Bookmark:
				if added != want {
					return fmt.Errorf("snapshot has %d objects, want %d", added, want)
				}
				return nil
			default:
				return fmt.Errorf("unexpected %s before bookmark", ev.Type)
			}
		case <-timeout:
			return errors.New("timed out waiting for bookmark")
		}
	}
}

// Check compares an error with the expected reason.
func Check(err error, want api.Reason) error {
	if want == "" {
		if err != nil {
			return fmt.Errorf("want success, got %s: %v", api.ReasonOf(err), err)
		}
		return nil
	}
	if err == nil {
		return fmt.Errorf("want %s, got success", want)
	}
	if got := api.ReasonOf(err); got != want {
		return fmt.Errorf("want %s, got %s: %v", want, got, err)
	}
	return nil
}
