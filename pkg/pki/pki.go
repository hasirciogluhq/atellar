// Package pki holds the cluster certificate authority and the API server's
// TLS certificate. The API server always serves TLS. Clients trust the
// cluster CA, which they either get as a file or fetch once and check
// against a pinned hash (sha256 of the CA public key), the way kubeadm's
// --discovery-token-ca-cert-hash works.
package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// ServerName is the DNS name every API server certificate carries and every
// client verifies, so certificates do not depend on the address a client
// dials.
const ServerName = "atellar-apiserver"

const (
	DefaultDir = "/etc/atellar/pki"

	CACertFile     = "ca.crt"
	CAKeyFile      = "ca.key"
	ServerCertFile = "server.crt"
	ServerKeyFile  = "server.key"

	caValidity     = 10 * 365 * 24 * time.Hour
	serverValidity = 365 * 24 * time.Hour
	// renewBefore reissues the server certificate this long before expiry.
	renewBefore = 30 * 24 * time.Hour

	hashPrefix = "sha256:"
)

// CA is a certificate authority with its private key.
type CA struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	PEM  []byte
}

// NewCA creates a self-signed CA.
func NewCA(now time.Time) (*CA, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "atellar-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := encodeKey(key)
	if err != nil {
		return nil, nil, err
	}
	return &CA{Cert: cert, Key: key, PEM: encodeCert(der)}, keyPEM, nil
}

// IssueServer signs a server certificate for ServerName plus sans (DNS names
// or IP addresses).
func (ca *CA) IssueServer(sans []string, now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: ServerName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(serverValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{ServerName},
	}
	for _, san := range sans {
		san = strings.TrimSpace(san)
		switch ip := net.ParseIP(san); {
		case san == "":
		case ip != nil:
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		case !slices.Contains(tmpl.DNSNames, san):
			tmpl.DNSNames = append(tmpl.DNSNames, san)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err = encodeKey(key)
	if err != nil {
		return nil, nil, err
	}
	return encodeCert(der), keyPEM, nil
}

// Hash returns the pin of a CA certificate: "sha256:" and the hex SHA-256 of
// its DER encoded public key.
func Hash(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hashPrefix + hex.EncodeToString(sum[:])
}

// HashPEM returns the pin of the first certificate in a PEM bundle.
func HashPEM(caPEM []byte) (string, error) {
	cert, err := parseCert(caPEM)
	if err != nil {
		return "", err
	}
	return Hash(cert), nil
}

// ValidateHash checks the format of a pin.
func ValidateHash(h string) error {
	hexPart, ok := strings.CutPrefix(h, hashPrefix)
	if !ok {
		return fmt.Errorf("CA hash must start with %q", hashPrefix)
	}
	if b, err := hex.DecodeString(hexPart); err != nil || len(b) != sha256.Size {
		return errors.New("CA hash must be sha256: followed by 64 hex characters")
	}
	return nil
}

// Server is the API server's TLS identity. The certificate is reissued from
// the CA when it gets close to expiry, without a restart.
type Server struct {
	dir  string
	sans []string
	ca   *CA
	now  func() time.Time

	mu   sync.Mutex
	cert *tls.Certificate
}

// LoadOrCreateServer reads the CA and server certificate in dir and creates
// whatever is missing. The server certificate is reissued when it expires
// within 30 days or lacks one of sans.
func LoadOrCreateServer(dir string, sans []string) (*Server, error) {
	return loadOrCreateServer(dir, sans, time.Now)
}

func loadOrCreateServer(dir string, sans []string, now func() time.Time) (*Server, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	ca, err := loadOrCreateCA(dir, now())
	if err != nil {
		return nil, err
	}
	s := &Server{dir: dir, sans: sans, ca: ca, now: now}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// CAPEM is the CA certificate clients must trust.
func (s *Server) CAPEM() []byte { return s.ca.PEM }

// CAHash is the CA pin printed for `node join` and `config set-cluster`.
func (s *Server) CAHash() string { return Hash(s.ca.Cert) }

// TLSConfig serves the current certificate and renews it when needed.
func (s *Server) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.certificate()
		},
	}
}

func (s *Server) certificate() (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cert.Leaf.NotAfter.Sub(s.now()) > renewBefore {
		return s.cert, nil
	}
	// On failure keep serving the old certificate; it is still valid for
	// weeks and the next handshake retries.
	_ = s.issueLocked()
	return s.cert, nil
}

func (s *Server) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	certPEM, certErr := os.ReadFile(filepath.Join(s.dir, ServerCertFile))
	keyPEM, keyErr := os.ReadFile(filepath.Join(s.dir, ServerKeyFile))
	if certErr == nil && keyErr == nil {
		if cert, err := s.pair(certPEM, keyPEM); err == nil && s.usable(cert.Leaf) {
			s.cert = cert
			return nil
		}
	}
	return s.issueLocked()
}

func (s *Server) issueLocked() error {
	certPEM, keyPEM, err := s.ca.IssueServer(s.sans, s.now())
	if err != nil {
		return err
	}
	cert, err := s.pair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(s.dir, ServerKeyFile), keyPEM, 0o600); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(s.dir, ServerCertFile), certPEM, 0o644); err != nil {
		return err
	}
	s.cert = cert
	return nil
}

// pair builds the served chain: the server certificate followed by the CA,
// so a client that only has a pin can find and check the CA.
func (s *Server) pair(certPEM, keyPEM []byte) (*tls.Certificate, error) {
	chain := append(append([]byte{}, certPEM...), s.ca.PEM...)
	cert, err := tls.X509KeyPair(chain, keyPEM)
	if err != nil {
		return nil, err
	}
	if cert.Leaf == nil {
		if cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
			return nil, err
		}
	}
	return &cert, nil
}

// usable reports whether a stored certificate was signed by this CA, is far
// from expiry and covers every configured SAN.
func (s *Server) usable(leaf *x509.Certificate) bool {
	if leaf.CheckSignatureFrom(s.ca.Cert) != nil || leaf.NotAfter.Sub(s.now()) <= renewBefore {
		return false
	}
	if leaf.VerifyHostname(ServerName) != nil {
		return false
	}
	for _, san := range s.sans {
		if san = strings.TrimSpace(san); san != "" && leaf.VerifyHostname(san) != nil {
			return false
		}
	}
	return true
}

func loadOrCreateCA(dir string, now time.Time) (*CA, error) {
	certPath, keyPath := filepath.Join(dir, CACertFile), filepath.Join(dir, CAKeyFile)
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	switch {
	case certErr == nil && keyErr == nil:
		return parseCA(certPEM, keyPEM)
	case errors.Is(certErr, os.ErrNotExist) && errors.Is(keyErr, os.ErrNotExist):
	case certErr != nil:
		return nil, fmt.Errorf("read %s: %w", certPath, certErr)
	default:
		return nil, fmt.Errorf("read %s: %w", keyPath, keyErr)
	}
	ca, keyPEM, err := NewCA(now)
	if err != nil {
		return nil, err
	}
	if err := writeFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, err
	}
	if err := writeFile(certPath, ca.PEM, 0o644); err != nil {
		return nil, err
	}
	return ca, nil
}

func parseCA(certPEM, keyPEM []byte) (*CA, error) {
	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("ca key: no PEM block")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ca key: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("ca key cannot sign")
	}
	return &CA{Cert: cert, Key: signer, PEM: encodeCert(cert.Raw)}, nil
}

// ClientConfig trusts only the cluster CA and verifies ServerName.
func ClientConfig(caPEM []byte) (*tls.Config, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("no CA certificate found")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, ServerName: ServerName}, nil
}

// FetchCA connects to addr ("host:port"), finds the CA in the chain the
// server presents, checks it against the pin and that it signed the server
// certificate for ServerName, and returns it as PEM. This is the only
// connection made without a trusted CA, and the pin makes it safe.
func FetchCA(ctx context.Context, addr, pin string) ([]byte, error) {
	if err := ValidateHash(pin); err != nil {
		return nil, err
	}
	var caPEM []byte
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: ServerName,
		// The chain is verified below against the pinned CA instead of the
		// system roots.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			ca, err := verifyPinned(cs.PeerCertificates, pin)
			if err != nil {
				return err
			}
			caPEM = encodeCert(ca.Raw)
			return nil
		},
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: cfg}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("fetch CA from %s: %w", addr, err)
	}
	_ = conn.Close()
	return caPEM, nil
}

func verifyPinned(chain []*x509.Certificate, pin string) (*x509.Certificate, error) {
	if len(chain) == 0 {
		return nil, errors.New("server sent no certificate")
	}
	for _, c := range chain[1:] {
		if !c.IsCA || subtle.ConstantTimeCompare([]byte(Hash(c)), []byte(pin)) != 1 {
			continue
		}
		roots := x509.NewCertPool()
		roots.AddCert(c)
		if _, err := chain[0].Verify(x509.VerifyOptions{Roots: roots, DNSName: ServerName}); err != nil {
			return nil, fmt.Errorf("server certificate is not signed by the pinned CA: %w", err)
		}
		return c, nil
	}
	return nil, errors.New("server CA does not match the pinned hash")
}

func parseCert(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("no PEM certificate found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func encodeCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func encodeKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		panic(err)
	}
	return n
}

// writeFile replaces path atomically.
func writeFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
