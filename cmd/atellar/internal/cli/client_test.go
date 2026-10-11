package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/host"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

// cli runs the CLI against a fixture served on both transports.
type cli struct {
	t          *testing.T
	f          *apiservertest.Fixture
	configPath string
	ports      map[transport.Kind]int
}

func newCLI(t *testing.T) *cli {
	f := apiservertest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ports := map[transport.Kind]int{}
	for _, kind := range []transport.Kind{transport.HTTP, transport.GRPC} {
		srv, _ := transport.NewServer(kind, f.Server, f.Auth)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ports[kind] = ln.Addr().(*net.TCPAddr).Port
		go func() { _ = srv.Serve(ctx, ln) }()
	}
	c := &cli{t: t, f: f, configPath: filepath.Join(t.TempDir(), "config"), ports: ports}
	for _, kind := range []string{"http", "grpc"} {
		c.ok("config", "set-cluster", kind, "--address", "127.0.0.1",
			"--http-port", strconv.Itoa(ports[transport.HTTP]), "--grpc-port", strconv.Itoa(ports[transport.GRPC]),
			"--transport", kind, "--token", apiservertest.AdminToken)
		c.ok("config", "set-context", kind, "--cluster", kind)
	}
	return c
}

func (c *cli) run(args ...string) (string, error) {
	var out bytes.Buffer
	root := NewRoot(&out, host.Host{Root: c.t.TempDir()})
	root.SetArgs(append([]string{"--config", c.configPath}, args...))
	err := root.Execute()
	return out.String(), err
}

func (c *cli) ok(args ...string) string {
	c.t.Helper()
	out, err := c.run(args...)
	if err != nil {
		c.t.Fatalf("atellar %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func TestConfigCommands(t *testing.T) {
	c := newCLI(t)
	if _, err := c.run("nodes", "list"); err == nil || !strings.Contains(err.Error(), "no current context") {
		t.Fatalf("no context: %v", err)
	}
	c.ok("config", "use-context", "grpc")
	if out := c.ok("config", "current-context"); strings.TrimSpace(out) != "grpc" {
		t.Fatalf("current-context = %q", out)
	}
	out := c.ok("config", "get-contexts")
	if !strings.Contains(out, "*        grpc") && !strings.Contains(out, "*") {
		t.Fatalf("get-contexts:\n%s", out)
	}
	if _, err := c.run("config", "use-context", "missing"); err == nil {
		t.Fatal("missing context accepted")
	}
}

// TestCommandsOverBothTransports runs every operator command through each
// transport selected by context.
func TestCommandsOverBothTransports(t *testing.T) {
	for _, kind := range []string{"http", "grpc"} {
		t.Run(kind, func(t *testing.T) {
			c := newCLI(t)
			ctx := []string{"--context", kind}
			run := func(args ...string) string { return c.ok(append(ctx, args...)...) }

			if out := run("nodes", "list"); !strings.Contains(out, "node-a") || !strings.Contains(out, "node-b") {
				t.Fatalf("nodes list:\n%s", out)
			}
			if out := run("nodes", "get", "node-a"); !strings.Contains(out, c.f.NodeA.ID) {
				t.Fatalf("nodes get:\n%s", out)
			}
			if out := run("nodes", "overlay-ips", "node-a", "--free"); !strings.Contains(out, "IP") {
				t.Fatalf("overlay-ips:\n%s", out)
			}

			var created v1.Container
			out := run("-o", "json", "containers", "create", "--image", "docker.io/library/redis:7",
				"--env", "A=1", "--cpu", "0.5", "--memory", "64", "--restart", "always", "--command", "redis-server --save 60 1")
			if err := json.Unmarshal([]byte(out), &created); err != nil {
				t.Fatalf("create output: %v\n%s", err, out)
			}
			if created.Env["A"] != "1" || *created.CpuLimit != 0.5 || *created.MemoryLimitMiB != 64 ||
				created.RestartPolicy != v1.RestartPolicyAlways || len(created.Command) != 4 {
				t.Fatalf("created %+v", created)
			}
			if out := run("containers", "list", "--active"); !strings.Contains(out, created.ID) {
				t.Fatalf("containers list:\n%s", out)
			}
			run("containers", "delete", created.ID)
			if out := run("containers", "events", created.ID); !strings.Contains(out, "scheduled") || !strings.Contains(out, "removed") {
				t.Fatalf("events:\n%s", out)
			}

			if out := run("join-tokens", "create", "--multi-use"); !strings.Contains(out, "atellar node join --join-token ") {
				t.Fatalf("join-tokens create:\n%s", out)
			}
			if out := run("join-tokens", "list"); strings.Count(out, "jtok_") < 1 {
				t.Fatalf("join-tokens list:\n%s", out)
			}
			if out := run("nodes", "evict", "node-b"); !strings.Contains(out, "evicted") {
				t.Fatalf("evict:\n%s", out)
			}

			if _, err := c.run(append(ctx, "containers", "create", "--image", "x", "--env", "novalue")...); err == nil {
				t.Fatal("bad --env accepted")
			}
			if _, err := c.run(append(ctx, "-o", "yaml", "nodes", "list")...); err == nil {
				t.Fatal("unknown output format accepted")
			}
		})
	}
}

func TestWrongToken(t *testing.T) {
	c := newCLI(t)
	c.ok("config", "set-cluster", "bad", "--address", "127.0.0.1", "--grpc-port", strconv.Itoa(c.ports[transport.GRPC]), "--token", "wrong")
	c.ok("config", "set-context", "bad", "--cluster", "bad")
	if _, err := c.run("--context", "bad", "nodes", "list"); err == nil || !strings.Contains(err.Error(), "invalid credential") {
		t.Fatalf("wrong token: %v", err)
	}
}
