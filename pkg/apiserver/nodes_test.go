package apiserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/auth/token"
	"github.com/hasirciogluhq/atellar/pkg/storage/memory"
)

// TestRegisterJoinTokens covers every join token state against first and
// second use.
func TestRegisterJoinTokens(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	cases := []struct {
		name       string
		singleUse  bool
		expiresAt  *time.Time
		plain      string // overrides the stored token when set
		wantFirst  api.Reason
		wantSecond api.Reason
	}{
		{name: "single use", singleUse: true, wantSecond: api.ReasonBadRequest},
		{name: "single use with future expiry", singleUse: true, expiresAt: &future, wantSecond: api.ReasonBadRequest},
		{name: "multi use", singleUse: false},
		{name: "multi use with future expiry", singleUse: false, expiresAt: &future},
		{name: "expired single use", singleUse: true, expiresAt: &past, wantFirst: api.ReasonBadRequest, wantSecond: api.ReasonBadRequest},
		{name: "expired multi use", singleUse: false, expiresAt: &past, wantFirst: api.ReasonBadRequest, wantSecond: api.ReasonBadRequest},
		{name: "unknown token", plain: "unknown", wantFirst: api.ReasonBadRequest, wantSecond: api.ReasonBadRequest},
		{name: "empty token", plain: " ", wantFirst: api.ReasonBadRequest, wantSecond: api.ReasonBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := apiservertest.New(t)
			ctx := context.Background()
			plain := "join-" + tc.name
			if _, err := f.Store.CreateJoinToken(ctx, token.Hash(plain), tc.singleUse, tc.expiresAt); err != nil {
				t.Fatal(err)
			}
			if tc.plain != "" {
				plain = tc.plain
			}
			for i, want := range []api.Reason{tc.wantFirst, tc.wantSecond} {
				_, err := f.Server.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: plain})
				if err := apiservertest.Check(err, want); err != nil {
					t.Fatalf("use %d: %v", i+1, err)
				}
			}
		})
	}
}

func TestRegisterValidation(t *testing.T) {
	f := apiservertest.New(t)
	ctx := context.Background()
	cases := map[string]v1.RegisterNodeRequest{
		"bad public ip":  {JoinToken: f.JoinToken, PublicIP: "300.1.1.1"},
		"bad private ip": {JoinToken: f.JoinToken, PrivateIP: "nope"},
		"no token":       {},
	}
	for name, req := range cases {
		_, err := f.Server.Nodes().Register(ctx, req)
		if err := apiservertest.Check(err, api.ReasonBadRequest); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// A failed registration must not consume the token.
	if _, err := f.Server.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: f.JoinToken, Name: "node-a"}); api.ReasonOf(err) != api.ReasonConflict {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.Server.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: f.JoinToken, Name: "node-c"}); err != nil {
		t.Fatalf("token was consumed by a failed registration: %v", err)
	}
}

// TestRegisterSubnets checks every node gets its own subnet, the node IP and
// pool live inside it, and the cluster CIDR runs out cleanly.
func TestRegisterSubnets(t *testing.T) {
	store := memory.New()
	srv, err := apiserver.New(apiserver.Config{Store: store, ClusterCIDR: "10.9.0.0/22", NodeSubnetPrefixLen: 24})
	if err != nil {
		t.Fatal(err)
	}
	admin := authn.WithPrincipal(context.Background(), &authn.Principal{Subject: authn.SubjectAdmin, Scopes: []string{"cluster:admin", "cluster:read"}})
	seen := map[string]bool{}
	for i := 0; ; i++ {
		jt, err := srv.JoinTokens().Create(admin, v1.CreateJoinTokenRequest{})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := srv.Nodes().Register(context.Background(), v1.RegisterNodeRequest{JoinToken: jt.Token})
		if err != nil {
			if api.ReasonOf(err) != api.ReasonUnavailable {
				t.Fatalf("register %d: %v", i, err)
			}
			if len(seen) != 4 {
				t.Fatalf("ran out after %d subnets, want 4", len(seen))
			}
			break
		}
		node := resp.Node
		if node.Status != v1.NodeStatusReady || seen[node.OverlaySubnet] {
			t.Fatalf("node %d: status %s subnet %s (seen %v)", i, node.Status, node.OverlaySubnet, seen[node.OverlaySubnet])
		}
		seen[node.OverlaySubnet] = true
		_, subnet, err := net.ParseCIDR(node.OverlaySubnet)
		if err != nil || !subnet.Contains(net.ParseIP(node.OverlayIP)) {
			t.Fatalf("node ip %s outside subnet %s", node.OverlayIP, node.OverlaySubnet)
		}
		pool, err := srv.Nodes().ListOverlayIPs(admin, node.ID, true)
		if err != nil || len(pool) == 0 {
			t.Fatalf("pool: %d entries, %v", len(pool), err)
		}
		for _, ip := range pool {
			if !subnet.Contains(net.ParseIP(ip.IP)) || ip.IP == node.OverlayIP {
				t.Fatalf("pool ip %s invalid for subnet %s", ip.IP, node.OverlaySubnet)
			}
		}
	}
}

// TestEvictReclaimsSubnet checks an evicted node's subnet goes to the next
// node and its credential stops working.
func TestEvictReclaimsSubnet(t *testing.T) {
	f := apiservertest.New(t)
	ctx := f.Context(t, apiservertest.Admin)
	evicted, err := f.Server.Nodes().Evict(ctx, f.NodeB.Name)
	if err != nil || evicted.Status != v1.NodeStatusEvicted {
		t.Fatalf("evict: %v %v", evicted, err)
	}
	if again, err := f.Server.Nodes().Evict(ctx, f.NodeB.ID); err != nil || again.Status != v1.NodeStatusEvicted {
		t.Fatalf("second evict must be a no-op: %v", err)
	}
	if _, err := f.Auth.Authenticate(ctx, f.KeyB); err == nil {
		t.Fatal("evicted node key still authenticates")
	}

	resp, err := f.Server.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: f.JoinToken, Name: "node-c"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Node.OverlaySubnet != f.NodeB.OverlaySubnet {
		t.Fatalf("node-c subnet = %s, want reclaimed %s", resp.Node.OverlaySubnet, f.NodeB.OverlaySubnet)
	}
	old, err := f.Server.Nodes().Get(ctx, f.NodeB.ID)
	if err != nil || old.OverlaySubnet != "" {
		t.Fatalf("evicted node keeps subnet %q (%v)", old.OverlaySubnet, err)
	}
}

// TestHeartbeatTransitions checks heartbeats publish only the not_ready to
// ready transition.
func TestHeartbeatTransitions(t *testing.T) {
	cases := []struct {
		from        v1.NodeStatus
		want        v1.NodeStatus
		wantPublish bool
	}{
		{v1.NodeStatusReady, v1.NodeStatusReady, false},
		{v1.NodeStatusNotReady, v1.NodeStatusReady, true},
		{v1.NodeStatusMaintenance, v1.NodeStatusMaintenance, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.from), func(t *testing.T) {
			f := apiservertest.New(t)
			ctx := f.Context(t, apiservertest.NodeSelf)
			if _, err := f.Store.SetNodeStatus(ctx, f.NodeA.ID, tc.from); err != nil {
				t.Fatal(err)
			}
			sub := f.Server.NodeEvents().Subscribe(nil)
			defer sub.Cancel()
			if err := f.Server.Nodes().SelfHeartbeat(ctx); err != nil {
				t.Fatal(err)
			}
			node, _ := f.Store.GetNode(ctx, f.NodeA.ID)
			if node.Status != tc.want || node.LastHeartbeat == nil {
				t.Fatalf("status %s heartbeat %v, want %s", node.Status, node.LastHeartbeat, tc.want)
			}
			select {
			case ev := <-sub.C():
				if !tc.wantPublish || ev.Type != watch.Modified {
					t.Fatalf("unexpected event %s", ev.Type)
				}
			default:
				if tc.wantPublish {
					t.Fatal("no event published")
				}
			}
		})
	}
}

func TestSelfRenewAPIKey(t *testing.T) {
	f := apiservertest.New(t)
	ctx := f.Context(t, apiservertest.NodeSelf)
	key, err := f.Server.Nodes().SelfRenewAPIKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if key.NodeAPIKey == f.KeyA || time.Until(key.APIKeyExpiresAt) < token.NodeAPIKeyTTL-time.Minute {
		t.Fatalf("renewed key %+v", key)
	}
	if _, err := f.Auth.Authenticate(ctx, f.KeyA); err == nil {
		t.Fatal("old key still valid")
	}
	p, err := f.Auth.Authenticate(ctx, key.NodeAPIKey)
	if err != nil || p.Node.ID != f.NodeA.ID {
		t.Fatalf("new key: %+v %v", p, err)
	}
}

func TestSelfReportHardware(t *testing.T) {
	f := apiservertest.New(t)
	ctx := f.Context(t, apiservertest.NodeSelf)
	if err := f.Server.Nodes().SelfReportHardware(ctx, v1.NodeHardware{}); api.ReasonOf(err) != api.ReasonBadRequest {
		t.Fatalf("empty hardware: %v", err)
	}
	if err := f.Server.Nodes().SelfReportHardware(ctx, v1.NodeHardware{CpuCores: 16, MemoryTotalMiB: 65536, Hostname: "a"}); err != nil {
		t.Fatal(err)
	}
	node, _ := f.Store.GetNode(ctx, f.NodeA.ID)
	if *node.CpuCores != 16 || *node.Hostname != "a" {
		t.Fatalf("hardware not stored: %+v", node)
	}
}

func TestNodeRefs(t *testing.T) {
	f := apiservertest.New(t)
	ctx := f.Context(t, apiservertest.Admin)
	for _, ref := range []string{f.NodeA.ID, f.NodeA.Name} {
		n, err := f.Server.Nodes().Get(ctx, ref)
		if err != nil || n.ID != f.NodeA.ID {
			t.Errorf("Get(%q) = %v, %v", ref, n, err)
		}
	}
	for _, ref := range []string{"node_missing", "missing"} {
		if _, err := f.Server.Nodes().Get(ctx, ref); !api.IsNotFound(err) {
			t.Errorf("Get(%q) error = %v", ref, err)
		}
	}
}
