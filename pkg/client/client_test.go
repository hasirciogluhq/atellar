package client

import (
	"crypto/tls"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/transport"
)

func TestControlPlane(t *testing.T) {
	cp := ControlPlane{Address: "10.1.2.3", HTTPPort: 8080, GRPCPort: 9090}
	if err := cp.Validate(); err != nil {
		t.Fatal(err)
	}
	if cp.HTTPBaseURL() != "https://10.1.2.3:8080" || cp.GRPCAddr() != "10.1.2.3:9090" {
		t.Fatalf("%s %s", cp.HTTPBaseURL(), cp.GRPCAddr())
	}
	v6 := ControlPlane{Address: "fd00::1", HTTPPort: 1, GRPCPort: 2}
	if v6.HTTPBaseURL() != "https://[fd00::1]:1" || v6.GRPCAddr() != "[fd00::1]:2" {
		t.Fatalf("ipv6: %s %s", v6.HTTPBaseURL(), v6.GRPCAddr())
	}
	for name, bad := range map[string]ControlPlane{
		"no address":   {HTTPPort: 1, GRPCPort: 2},
		"no http port": {Address: "a", GRPCPort: 2},
		"no grpc port": {Address: "a", HTTPPort: 1},
		"port range":   {Address: "a", HTTPPort: 70000, GRPCPort: 2},
	} {
		if bad.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestNewSelectsTransport(t *testing.T) {
	cp := ControlPlane{Address: "127.0.0.1", HTTPPort: 8080, GRPCPort: 9090}
	for _, tc := range []struct {
		transport string
		ok        bool
	}{{"", true}, {"grpc", true}, {"http", true}, {"tcp", false}, {"udp", false}} {
		c, err := New(Config{ControlPlane: cp, Transport: tc.transport, TLS: &tls.Config{}})
		if (err == nil) != tc.ok {
			t.Errorf("transport %q: err = %v", tc.transport, err)
		}
		if c != nil {
			_ = c.Close()
		}
	}
	if _, err := New(Config{Transport: "grpc", TLS: &tls.Config{}}); err == nil {
		t.Error("empty control plane accepted")
	}
	if _, err := New(Config{ControlPlane: cp}); err == nil {
		t.Error("missing TLS configuration accepted")
	}
	if e, _ := cp.Endpoint(transport.HTTP); e != "https://127.0.0.1:8080" {
		t.Errorf("endpoint %s", e)
	}
}
