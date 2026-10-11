package pki

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateServerReusesCA(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateServer(dir, []string{"10.0.0.10"})
	if err != nil {
		t.Fatal(err)
	}
	for file, mode := range map[string]os.FileMode{CACertFile: 0o644, CAKeyFile: 0o600, ServerCertFile: 0o644, ServerKeyFile: 0o600} {
		info, err := os.Stat(filepath.Join(dir, file))
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s: mode %v, %v", file, info.Mode(), err)
		}
	}
	certBefore, _ := os.ReadFile(filepath.Join(dir, ServerCertFile))

	second, err := LoadOrCreateServer(dir, []string{"10.0.0.10"})
	if err != nil {
		t.Fatal(err)
	}
	if first.CAHash() != second.CAHash() {
		t.Fatal("CA changed on reload")
	}
	if certAfter, _ := os.ReadFile(filepath.Join(dir, ServerCertFile)); string(certAfter) != string(certBefore) {
		t.Fatal("valid server certificate was reissued")
	}

	// A new SAN reissues the server certificate but keeps the CA.
	third, err := LoadOrCreateServer(dir, []string{"10.0.0.10", "api.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if third.CAHash() != first.CAHash() {
		t.Fatal("CA changed when a SAN was added")
	}
	leaf := third.cert.Leaf
	for _, name := range []string{ServerName, "api.example.com", "10.0.0.10"} {
		if err := leaf.VerifyHostname(name); err != nil {
			t.Errorf("certificate does not cover %s: %v", name, err)
		}
	}
}

func TestLoadOrCreateServerRejectsHalfCA(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CACertFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateServer(dir, nil); err == nil {
		t.Fatal("a CA certificate without its key was replaced silently")
	}
}

func TestServerRenewsBeforeExpiry(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	s, err := loadOrCreateServer(t.TempDir(), nil, clock)
	if err != nil {
		t.Fatal(err)
	}
	issued := s.cert.Leaf.SerialNumber
	cfg := s.TLSConfig()

	got, err := cfg.GetCertificate(nil)
	if err != nil || got.Leaf.SerialNumber.Cmp(issued) != 0 {
		t.Fatalf("fresh certificate replaced: %v", err)
	}
	now = now.Add(serverValidity - renewBefore + time.Hour)
	got, err = cfg.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Leaf.SerialNumber.Cmp(issued) == 0 {
		t.Fatal("certificate not renewed inside the renewal window")
	}
	if got.Leaf.NotAfter.Sub(now) <= renewBefore {
		t.Fatalf("renewed certificate expires %s", got.Leaf.NotAfter)
	}
}

func TestHash(t *testing.T) {
	s, err := LoadOrCreateServer(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err := HashPEM(s.CAPEM())
	if err != nil || h != s.CAHash() {
		t.Fatalf("HashPEM = %s, %v; want %s", h, err, s.CAHash())
	}
	if err := ValidateHash(h); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "abc", "sha256:", "sha256:zz", "md5:" + h[7:], h[:len(h)-2]} {
		if ValidateHash(bad) == nil {
			t.Errorf("ValidateHash(%q) accepted", bad)
		}
	}
	if _, err := HashPEM([]byte("junk")); err == nil {
		t.Error("HashPEM accepted junk")
	}
}

// serve answers TLS handshakes with cfg until the test ends.
func serve(t *testing.T, cfg *tls.Config) string {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = conn.(*tls.Conn).Handshake()
				conn.Close()
			}()
		}
	}()
	return ln.Addr().String()
}

func TestFetchCAAndClientConfig(t *testing.T) {
	s, err := LoadOrCreateServer(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	addr := serve(t, s.TLSConfig())
	ctx := context.Background()

	caPEM, err := FetchCA(ctx, addr, s.CAHash())
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := HashPEM(caPEM); h != s.CAHash() {
		t.Fatalf("fetched CA %s", h)
	}

	other, err := LoadOrCreateServer(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FetchCA(ctx, addr, other.CAHash()); err == nil {
		t.Fatal("FetchCA accepted a wrong pin")
	}
	if _, err := FetchCA(ctx, addr, "sha256:nope"); err == nil {
		t.Fatal("FetchCA accepted a malformed pin")
	}

	// The fetched CA verifies the server; another CA does not.
	good, err := ClientConfig(caPEM)
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := tls.Dial("tcp", addr, good); err != nil {
		t.Fatalf("dial with cluster CA: %v", err)
	} else {
		conn.Close()
	}
	bad, _ := ClientConfig(other.CAPEM())
	if conn, err := tls.Dial("tcp", addr, bad); err == nil {
		conn.Close()
		t.Fatal("dial with a foreign CA succeeded")
	}
	if _, err := ClientConfig([]byte("junk")); err == nil {
		t.Fatal("ClientConfig accepted junk")
	}
}

// TestFetchCARejectsBorrowedCA covers a server that presents the genuine CA
// certificate in its chain but a leaf signed by a different key: knowing
// the public CA certificate must not be enough to pass the pin.
func TestFetchCARejectsBorrowedCA(t *testing.T) {
	genuine, err := LoadOrCreateServer(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	attacker, _, err := NewCA(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM, err := attacker.IssueServer(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	chain := append(append([]byte{}, certPEM...), genuine.CAPEM()...)
	cert, err := tls.X509KeyPair(chain, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	addr := serve(t, &tls.Config{Certificates: []tls.Certificate{cert}})
	if _, err := FetchCA(context.Background(), addr, genuine.CAHash()); err == nil {
		t.Fatal("FetchCA trusted a leaf the pinned CA did not sign")
	}
}

func TestIssueServerSANs(t *testing.T) {
	ca, _, err := NewCA(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _, err := ca.IssueServer([]string{"", " 192.168.1.5 ", "api.local", ServerName}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	cert, err := parseCert(certPEM)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.DNSNames) != 2 || len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(net.ParseIP("192.168.1.5")) {
		t.Fatalf("SANs: %v %v", cert.DNSNames, cert.IPAddresses)
	}
	if cert.IsCA || cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatal("server certificate has CA rights or lacks server auth")
	}
}
