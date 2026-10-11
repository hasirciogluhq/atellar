package agentconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/pki/pkitest"
)

func valid(t *testing.T) *Config {
	return &Config{
		ControlPlane:    client.ControlPlane{Address: "10.0.0.1", HTTPPort: 8080, GRPCPort: 9090},
		CACert:          string(pkitest.New(t).Server.CAPEM()),
		NodeID:          "node_1",
		NodeAPIKey:      "key",
		APIKeyExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Network:         Network{MTU: 1400, ReconcileInterval: Duration(time.Minute)},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "etc", "agent.json")
	if err := Save(path, valid(t)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, %v", info.Mode(), err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"reconcile_interval": "1m0s"`) {
		t.Fatalf("durations must be strings:\n%s", data)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "grpc" || cfg.ContainerdSock != DefaultContainerdSock || time.Duration(cfg.HeartbeatInterval) != DefaultHeartbeatInterval {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
	if cfg.Network.MTU != 1400 || time.Duration(cfg.Network.ReconcileInterval) != time.Minute {
		t.Fatalf("network lost: %+v", cfg.Network)
	}
	if _, err := cfg.TLS(); err != nil {
		t.Fatalf("TLS: %v", err)
	}
	cfg.CACert = "not a certificate"
	if _, err := cfg.TLS(); err == nil {
		t.Fatal("TLS accepted a broken CA")
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]func(*Config){
		"no node id":    func(c *Config) { c.NodeID = "" },
		"no key":        func(c *Config) { c.NodeAPIKey = "" },
		"no expiry":     func(c *Config) { c.APIKeyExpiresAt = time.Time{} },
		"no address":    func(c *Config) { c.ControlPlane.Address = "" },
		"bad transport": func(c *Config) { c.Transport = "carrier-pigeon" },
		"no ca cert":    func(c *Config) { c.CACert = "" },
	}
	for name, mutate := range cases {
		cfg := valid(t)
		cfg.SetDefaults()
		mutate(cfg)
		if cfg.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil || !strings.Contains(err.Error(), "atellar node join") {
		t.Errorf("missing file: %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte(`{"heartbeat_interval":"soon"}`), 0o600)
	if _, err := Load(bad); err == nil {
		t.Error("bad duration accepted")
	}
}
