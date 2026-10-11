package agent

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/config/agentconfig"
	"github.com/hasirciogluhq/atellar/pkg/pki/pkitest"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

func TestRenewIfDue(t *testing.T) {
	for _, tc := range []struct {
		name    string
		expires time.Duration
		renew   bool
	}{
		{"far from expiry", 60 * 24 * time.Hour, false},
		{"inside renew window", 2 * 24 * time.Hour, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := apiservertest.New(t)
			certs := pkitest.New(t)
			srv, _ := transport.NewServer(transport.GRPC, f.Server, f.Auth, transport.WithTLS(certs.Server.TLSConfig()))
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _ = srv.Serve(ctx, ln) }()

			path := filepath.Join(t.TempDir(), "agent.json")
			cfg := &agentconfig.Config{
				ControlPlane:    client.ControlPlane{Address: "127.0.0.1", HTTPPort: 1, GRPCPort: 1},
				CACert:          string(certs.Server.CAPEM()),
				NodeID:          f.NodeA.ID,
				NodeAPIKey:      f.KeyA,
				APIKeyExpiresAt: time.Now().Add(tc.expires),
			}
			cfg.SetDefaults()
			a := New(path, cfg)
			c, err := transport.NewClient(transport.GRPC, ln.Addr().String(), a.apiKey, transport.WithTLS(certs.Client))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()

			if err := a.renewIfDue(ctx, c.Nodes()); err != nil {
				t.Fatal(err)
			}
			renewed := a.apiKey() != f.KeyA
			if renewed != tc.renew {
				t.Fatalf("renewed = %v, want %v", renewed, tc.renew)
			}
			if !tc.renew {
				return
			}
			saved, err := agentconfig.Load(path)
			if err != nil || saved.NodeAPIKey != a.apiKey() {
				t.Fatalf("saved config: %v", err)
			}
			// The client sends the new key on the very next call.
			if err := c.Nodes().SelfHeartbeat(ctx); err != nil {
				t.Fatalf("heartbeat with renewed key: %v", err)
			}
		})
	}
}
