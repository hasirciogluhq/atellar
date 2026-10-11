package serverconfig

import (
	"strings"
	"testing"
	"time"
)

func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := env[k]; return v, ok }
}

func TestFromEnvDefaults(t *testing.T) {
	cfg, err := FromEnv(lookup(map[string]string{EnvDatabaseURL: "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr() != ":8080" || cfg.GRPCAddr() != ":9090" || cfg.ClusterCIDR != "10.0.0.0/8" ||
		cfg.NodeSubnetPrefixLen != 24 || cfg.NodeStaleAfter != 30*time.Second || cfg.AdminToken != "" {
		t.Fatalf("defaults: %+v", cfg)
	}
}

func TestFromEnvErrors(t *testing.T) {
	base := map[string]string{EnvDatabaseURL: "postgres://x"}
	cases := map[string]map[string]string{
		"no database":      {},
		"bad port":         {EnvHTTPPort: "eighty"},
		"port range":       {EnvGRPCPort: "70000"},
		"same ports":       {EnvHTTPPort: "9000", EnvGRPCPort: "9000"},
		"bad cidr":         {EnvClusterCIDR: "10.0.0.0"},
		"ipv6 cidr":        {EnvClusterCIDR: "fd00::/64"},
		"prefix too short": {EnvClusterCIDR: "10.0.0.0/16", EnvNodeSubnetPrefixLen: "8"},
		"prefix too long":  {EnvNodeSubnetPrefixLen: "31"},
		"bad stale after":  {EnvNodeStaleAfter: "later"},
		"zero stale after": {EnvNodeStaleAfter: "0s"},
		"empty migrations": {EnvMigrationsPath: ""},
	}
	for name, overrides := range cases {
		env := map[string]string{}
		if name != "no database" {
			for k, v := range base {
				env[k] = v
			}
		}
		for k, v := range overrides {
			env[k] = v
		}
		if _, err := FromEnv(lookup(env)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestEnvRoundTrip(t *testing.T) {
	in := Default()
	in.DatabaseURL = "postgres://u:p@db:5432/atellar?sslmode=disable"
	in.AdminToken = "secret"
	in.BindAddress = "10.0.0.5"
	in.NodeStaleAfter = time.Minute
	rendered := in.Env()
	if strings.Contains(rendered, "\u2014") {
		t.Fatal("env file contains an em dash")
	}
	out, err := FromEnv(lookup(ParseEnv(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip:\n in %+v\nout %+v", in, out)
	}
}
