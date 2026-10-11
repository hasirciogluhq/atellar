// Package pkitest creates throwaway cluster PKI for tests.
package pkitest

import (
	"crypto/tls"
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/pki"
)

// PKI is a server identity and the client configuration that trusts it.
type PKI struct {
	Server *pki.Server
	Client *tls.Config
}

// New creates a CA and server certificate in a temporary directory.
func New(t testing.TB) *PKI {
	t.Helper()
	srv, err := pki.LoadOrCreateServer(t.TempDir(), []string{"127.0.0.1", "localhost"})
	if err != nil {
		t.Fatalf("pki: %v", err)
	}
	client, err := pki.ClientConfig(srv.CAPEM())
	if err != nil {
		t.Fatalf("pki client: %v", err)
	}
	return &PKI{Server: srv, Client: client}
}
