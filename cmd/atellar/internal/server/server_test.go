package server

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/config/serverconfig"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/memory"
	"github.com/hasirciogluhq/atellar/pkg/storage/postgres"
)

func listen(t *testing.T) (net.Listener, int) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln, ln.Addr().(*net.TCPAddr).Port
}

// TestRun boots the whole process on loopback and drives one node through
// join, heartbeat and the lifecycle controller over both transports.
func TestRun(t *testing.T) {
	t.Run("memory", func(t *testing.T) { testRun(t, memory.New()) })
	t.Run("postgres", func(t *testing.T) {
		url := os.Getenv("ATELLAR_TEST_DATABASE_URL")
		if url == "" {
			t.Skip("ATELLAR_TEST_DATABASE_URL is not set")
		}
		ctx := context.Background()
		if err := postgres.Migrate(ctx, url, "../../../../pkg/storage/postgres/migrations"); err != nil {
			t.Fatal(err)
		}
		pool, err := pgxpool.New(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		if _, err := pool.Exec(ctx, `TRUNCATE container_events, overlay_ip_pool, containers, node_join_tokens, nodes CASCADE`); err != nil {
			t.Fatal(err)
		}
		testRun(t, postgres.New(pool))
	})
}

func testRun(t *testing.T, store storage.Interface) {
	cfg := serverconfig.Default()
	cfg.DatabaseURL = "unused"
	cfg.AdminToken = "admin"
	cfg.NodeStaleAfter = 300 * time.Millisecond
	httpLn, httpPort := listen(t)
	grpcLn, grpcPort := listen(t)
	cp := client.ControlPlane{Address: "127.0.0.1", HTTPPort: httpPort, GRPCPort: grpcPort}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, store, Listeners{HTTP: httpLn, GRPC: grpcLn}) }()

	for _, kind := range []string{"http", "grpc"} {
		anon, err := client.New(client.Config{ControlPlane: cp, Transport: kind})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := anon.Nodes().List(ctx); !api.IsUnauthorized(err) {
			t.Fatalf("%s: anonymous list: %v", kind, err)
		}
		admin, _ := client.New(client.Config{ControlPlane: cp, Transport: kind, Token: client.Static("admin")})
		jt, err := admin.JoinTokens().Create(ctx, v1.CreateJoinTokenRequest{})
		if err != nil {
			t.Fatalf("%s: create join token: %v", kind, err)
		}
		reg, err := anon.Nodes().Register(ctx, v1.RegisterNodeRequest{JoinToken: jt.Token, Name: "node-" + kind, PrivateIP: "192.168.1.1"})
		if err != nil {
			t.Fatalf("%s: register: %v", kind, err)
		}
		agent, _ := client.New(client.Config{ControlPlane: cp, Transport: kind, Token: client.Static(reg.NodeAPIKey)})
		if err := agent.Nodes().SelfHeartbeat(ctx); err != nil {
			t.Fatalf("%s: heartbeat: %v", kind, err)
		}

		// Without heartbeats the controller marks the node not_ready.
		deadline := time.Now().Add(5 * time.Second)
		for {
			n, err := admin.Nodes().Get(ctx, reg.Node.ID)
			if err != nil {
				t.Fatal(err)
			}
			if n.Status == v1.NodeStatusNotReady {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: node still %s", kind, n.Status)
			}
			time.Sleep(50 * time.Millisecond)
		}
		if err := agent.Nodes().SelfHeartbeat(ctx); err != nil {
			t.Fatal(err)
		}
		if n, _ := admin.Nodes().Get(ctx, reg.Node.ID); n.Status != v1.NodeStatusReady {
			t.Fatalf("%s: heartbeat did not restore ready: %s", kind, n.Status)
		}
		_ = anon.Close()
		_ = admin.Close()
		_ = agent.Close()
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRunListenError(t *testing.T) {
	busy, port := listen(t)
	defer busy.Close()
	cfg := serverconfig.Default()
	cfg.BindAddress = "127.0.0.1"
	cfg.HTTPPort = port
	if err := Run(context.Background(), cfg, memory.New(), Listeners{}); err == nil {
		t.Fatal("Run succeeded on a busy port")
	}
}
