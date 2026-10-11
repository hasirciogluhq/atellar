package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/hasirciogluhq/atellar/pkg/pki"
)

// caSource is how a command learns the cluster CA: a PEM file, or a pin
// that the CA fetched from the API server must match. Both may be given;
// then the file must match the pin.
type caSource struct {
	file string
	hash string
}

// resolve returns the CA as PEM. host and port are the API server's HTTP
// endpoint; the gRPC port serves the same certificate.
func (s caSource) resolve(ctx context.Context, host string, port int) ([]byte, error) {
	switch {
	case s.file != "":
		caPEM, err := os.ReadFile(s.file)
		if err != nil {
			return nil, err
		}
		got, err := pki.HashPEM(caPEM)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.file, err)
		}
		if s.hash != "" && got != s.hash {
			return nil, fmt.Errorf("%s has hash %s, want %s", s.file, got, s.hash)
		}
		return caPEM, nil
	case s.hash != "":
		return pki.FetchCA(ctx, net.JoinHostPort(host, strconv.Itoa(port)), s.hash)
	}
	return nil, errors.New("--ca-cert-hash or --ca-cert is required")
}
