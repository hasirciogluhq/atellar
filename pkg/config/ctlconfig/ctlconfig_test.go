package ctlconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/client"
)

func TestContexts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Current(); err == nil {
		t.Fatal("empty config has a current cluster")
	}

	prod := Cluster{ControlPlane: client.ControlPlane{Address: "10.0.0.1", HTTPPort: 8080, GRPCPort: 9090}, Token: "t"}
	if err := cfg.SetCluster("prod", prod); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetCluster("", prod); err == nil {
		t.Fatal("unnamed cluster accepted")
	}
	if err := cfg.SetCluster("bad", Cluster{ControlPlane: prod.ControlPlane, Transport: "smtp"}); err == nil {
		t.Fatal("bad transport accepted")
	}
	if err := cfg.SetContext("p", "missing"); err == nil {
		t.Fatal("context to missing cluster accepted")
	}
	if err := cfg.SetContext("p", "prod"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UseContext("nope"); err == nil {
		t.Fatal("missing context accepted")
	}
	if err := cfg.UseContext("p"); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := loaded.Current()
	if err != nil || got != prod {
		t.Fatalf("Current = %+v, %v", got, err)
	}
	if names := loaded.ContextNames(); len(names) != 1 || names[0] != "p" {
		t.Fatalf("names %v", names)
	}
}
