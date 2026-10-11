package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hasirciogluhq/atellar/cmd/atellar/internal/host"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/apiservertest"
	"github.com/hasirciogluhq/atellar/pkg/config/agentconfig"
	"github.com/hasirciogluhq/atellar/pkg/config/serverconfig"
	"github.com/hasirciogluhq/atellar/pkg/pki"
	"github.com/hasirciogluhq/atellar/pkg/pki/pkitest"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

type machine struct {
	root  string
	calls []string
}

func newMachine(t *testing.T, files ...string) *machine {
	m := &machine{root: t.TempDir()}
	for _, f := range files {
		p := filepath.Join(m.root, f)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func (m *machine) run(args ...string) (string, error) {
	var out bytes.Buffer
	h := host.Host{Root: m.root, Systemctl: func(a ...string) error {
		m.calls = append(m.calls, strings.Join(a, " "))
		return nil
	}}
	root := NewRoot(&out, h)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func (m *machine) read(t *testing.T, path string) string {
	data, err := os.ReadFile(filepath.Join(m.root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestServerInstall(t *testing.T) {
	base := []string{"server", "install", "--database-url", "postgres://db/atellar", "--migrations-path", "/usr/share/atellar/migrations"}
	cases := []struct {
		name      string
		extra     []string
		wantToken func(string) bool
		wantErr   bool
	}{
		{"generated token", nil, func(tok string) bool { return len(tok) == 64 }, false},
		{"given token", []string{"--admin-token", "s3cret"}, func(tok string) bool { return tok == "s3cret" }, false},
		{"open mode is gone", []string{"--open"}, nil, true},
		{"bad cidr", []string{"--cluster-cidr", "nope"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMachine(t, host.Binary, "/usr/share/atellar/migrations/001_initial.sql")
			out, err := m.run(append(base, tc.extra...)...)
			if tc.wantErr {
				if err == nil {
					t.Fatal("install succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			env := m.read(t, serverconfig.EnvPath)
			cfg, err := serverconfig.FromEnv(func(k string) (string, bool) { v, ok := serverconfig.ParseEnv(env)[k]; return v, ok })
			if err != nil || !tc.wantToken(cfg.AdminToken) || cfg.DatabaseURL != "postgres://db/atellar" {
				t.Fatalf("env file: %+v %v", cfg, err)
			}
			if info, _ := os.Stat(filepath.Join(m.root, serverconfig.EnvPath)); info.Mode().Perm() != 0o600 {
				t.Fatalf("env file mode %v", info.Mode())
			}
			if unit := m.read(t, "/etc/systemd/system/"+host.APIUnit); !strings.Contains(unit, "EnvironmentFile="+serverconfig.EnvPath) {
				t.Fatalf("unit:\n%s", unit)
			}
			want := "daemon-reload,enable atellar-server.service,restart atellar-server.service"
			if got := strings.Join(m.calls, ","); got != want {
				t.Fatalf("systemctl calls %s", got)
			}
			if cfg.AdminToken != "" && tc.name == "generated token" && !strings.Contains(out, cfg.AdminToken) {
				t.Fatal("generated token not printed")
			}
			// The CA exists after install and its pin is printed for the CLI.
			caPEM, err := os.ReadFile(filepath.Join(m.root, cfg.PKIDir, pki.CACertFile))
			if err != nil {
				t.Fatal(err)
			}
			hash, _ := pki.HashPEM(caPEM)
			if !strings.Contains(out, "--ca-cert-hash "+hash) {
				t.Fatalf("CA pin %s not printed:\n%s", hash, out)
			}
			if info, _ := os.Stat(filepath.Join(m.root, cfg.PKIDir, pki.CAKeyFile)); info.Mode().Perm() != 0o600 {
				t.Fatalf("CA key mode %v", info.Mode())
			}
		})
	}
}

func TestServerInstallNeedsRelease(t *testing.T) {
	m := newMachine(t)
	if _, err := m.run("server", "install", "--database-url", "postgres://x"); err == nil || !strings.Contains(err.Error(), host.Binary) {
		t.Fatalf("missing binary: %v", err)
	}
}

// TestNodeJoin joins over each transport and checks the written config
// lets the agent authenticate.
func TestNodeJoin(t *testing.T) {
	for _, kind := range []transport.Kind{transport.HTTP, transport.GRPC} {
		for _, installed := range []bool{false, true} {
			t.Run(string(kind)+map[bool]string{false: "/no unit", true: "/unit installed"}[installed], func(t *testing.T) {
				f := apiservertest.New(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				certs := pkitest.New(t)
				ports := map[transport.Kind]string{}
				for _, k := range []transport.Kind{transport.HTTP, transport.GRPC} {
					srv, _ := transport.NewServer(k, f.Server, f.Auth, transport.WithTLS(certs.Server.TLSConfig()))
					ln, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					ports[k] = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
					go func() { _ = srv.Serve(ctx, ln) }()
				}

				m := newMachine(t, host.Binary)
				if installed {
					if _, err := m.run("node", "install"); err != nil {
						t.Fatal(err)
					}
				}
				m.calls = nil
				join := []string{"node", "join", "--join-token", f.JoinToken, "--name", "node-c", "--private-ip", "192.168.0.3",
					"--control-plane-address", "127.0.0.1", "--http-port", ports[transport.HTTP], "--grpc-port", ports[transport.GRPC],
					"--transport", string(kind), "--skip-containerd-check"}
				// Without a pin, or with someone else's, the node must not
				// talk to the server, and the token stays unused.
				if _, err := m.run(join...); err == nil || !strings.Contains(err.Error(), "--ca-cert-hash") {
					t.Fatalf("join without a CA: %v", err)
				}
				if _, err := m.run(append(join, "--ca-cert-hash", pkitest.New(t).Server.CAHash())...); err == nil {
					t.Fatal("join with a wrong CA pin succeeded")
				}
				out, err := m.run(append(join, "--ca-cert-hash", certs.Server.CAHash())...)
				if err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				cfg, err := agentconfig.Load(filepath.Join(m.root, agentconfig.Path))
				if err != nil {
					t.Fatal(err)
				}
				if cfg.NodeName != "node-c" || cfg.Transport != string(kind) {
					t.Fatalf("config %+v", cfg)
				}
				if hash, _ := pki.HashPEM([]byte(cfg.CACert)); hash != certs.Server.CAHash() {
					t.Fatalf("saved CA %s, want %s", hash, certs.Server.CAHash())
				}
				if p, err := f.Auth.Authenticate(ctx, cfg.NodeAPIKey); err != nil || p.Node.ID != cfg.NodeID {
					t.Fatalf("saved key does not authenticate: %v", err)
				}
				restarted := strings.Join(m.calls, ",") == "restart "+host.AgentUnit
				if restarted != installed {
					t.Fatalf("systemctl calls %v (unit installed %v)", m.calls, installed)
				}

				// renew-key replaces the saved key and restarts the agent.
				m.calls = nil
				if out, err := m.run("node", "renew-key"); err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				renewed, _ := agentconfig.Load(filepath.Join(m.root, agentconfig.Path))
				if renewed.NodeAPIKey == cfg.NodeAPIKey {
					t.Fatal("key not replaced")
				}
				if (len(m.calls) == 1) != installed {
					t.Fatalf("renew-key systemctl calls %v", m.calls)
				}

				// The token was single use.
				if _, err := m.run("node", "join", "--join-token", f.JoinToken, "--name", "node-d", "--private-ip", "192.168.0.4",
					"--control-plane-address", "127.0.0.1", "--http-port", ports[transport.HTTP], "--grpc-port", ports[transport.GRPC],
					"--ca-cert-hash", certs.Server.CAHash(), "--skip-containerd-check"); err == nil || !strings.Contains(err.Error(), "already used") {
					t.Fatalf("used token: %v", err)
				}

				// trust-ca restores a config that lost its CA, as on a node
				// joined before the server spoke TLS.
				cfg.CACert = ""
				data, _ := json.Marshal(cfg)
				if err := os.WriteFile(filepath.Join(m.root, agentconfig.Path), data, 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := agentconfig.Load(filepath.Join(m.root, agentconfig.Path)); err == nil || !strings.Contains(err.Error(), "trust-ca") {
					t.Fatalf("config without CA: %v", err)
				}
				if out, err := m.run("node", "trust-ca", "--ca-cert-hash", certs.Server.CAHash()); err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				if _, err := agentconfig.Load(filepath.Join(m.root, agentconfig.Path)); err != nil {
					t.Fatalf("after trust-ca: %v", err)
				}
			})
		}
	}
}

func TestNodeJoinNeedsContainerd(t *testing.T) {
	m := newMachine(t)
	_, err := m.run("node", "join", "--join-token", "x", "--name", "n", "--private-ip", "10.0.0.1",
		"--control-plane-address", "127.0.0.1", "--containerd-sock", filepath.Join(m.root, "missing.sock"))
	if err == nil || !strings.Contains(err.Error(), "containerd") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnitsRunSingleBinary(t *testing.T) {
	m := newMachine(t, host.Binary, "/usr/share/atellar/migrations/001_initial.sql")
	if _, err := m.run("server", "install", "--database-url", "postgres://x"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.run("node", "install"); err != nil {
		t.Fatal(err)
	}
	for unit, exec := range map[string]string{host.APIUnit: "server run", host.AgentUnit: "agent run"} {
		if body := m.read(t, "/etc/systemd/system/"+unit); !strings.Contains(body, "ExecStart="+host.Binary+" "+exec+"\n") {
			t.Errorf("%s:\n%s", unit, body)
		}
	}
}

// TestCommandTree checks that one binary carries every role.
func TestCommandTree(t *testing.T) {
	root := NewRoot(&bytes.Buffer{}, host.Host{})
	for _, path := range [][]string{
		{"config", "set-cluster"}, {"nodes", "list"}, {"nodes", "watch"}, {"containers", "create"},
		{"containers", "events"}, {"join-tokens", "create"}, {"server", "install"}, {"server", "run"},
		{"node", "install"}, {"node", "join"}, {"node", "renew-key"}, {"node", "trust-ca"}, {"agent", "run"}, {"version"},
	} {
		if c, _, err := root.Find(path); err != nil || c.Name() != path[len(path)-1] {
			t.Errorf("atellar %s missing", strings.Join(path, " "))
		}
	}
}
