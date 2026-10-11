// Package client connects a program to the Atellar API server over the
// configured transport. Every atellar subcommand uses it.
package client

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/hasirciogluhq/atellar/pkg/transport"
)

// ControlPlane is where the API server listens.
type ControlPlane struct {
	Address  string `json:"address"`
	HTTPPort int    `json:"http_port"`
	GRPCPort int    `json:"grpc_port"`
}

func (cp ControlPlane) Validate() error {
	if cp.Address == "" {
		return errors.New("control plane address is required")
	}
	if cp.HTTPPort <= 0 || cp.HTTPPort > 65535 {
		return errors.New("control plane http port must be between 1 and 65535")
	}
	if cp.GRPCPort <= 0 || cp.GRPCPort > 65535 {
		return errors.New("control plane grpc port must be between 1 and 65535")
	}
	return nil
}

func (cp ControlPlane) HTTPBaseURL() string {
	return "http://" + net.JoinHostPort(cp.Address, strconv.Itoa(cp.HTTPPort))
}

func (cp ControlPlane) GRPCAddr() string {
	return net.JoinHostPort(cp.Address, strconv.Itoa(cp.GRPCPort))
}

// Endpoint returns the address the given transport dials.
func (cp ControlPlane) Endpoint(kind transport.Kind) (string, error) {
	switch kind {
	case transport.HTTP:
		return cp.HTTPBaseURL(), nil
	case transport.GRPC, transport.TCP:
		return cp.GRPCAddr(), nil
	}
	return "", fmt.Errorf("unknown transport %q", kind)
}

type Config struct {
	ControlPlane ControlPlane
	// Transport is "http", "grpc" (the default) or "tcp".
	Transport string
	// Token returns the bearer token for each request; nil sends none.
	Token func() string
}

// New returns a connected api.Interface.
func New(cfg Config) (transport.Client, error) {
	if err := cfg.ControlPlane.Validate(); err != nil {
		return nil, err
	}
	kind, err := transport.ParseKind(cfg.Transport)
	if err != nil {
		return nil, err
	}
	endpoint, err := cfg.ControlPlane.Endpoint(kind)
	if err != nil {
		return nil, err
	}
	return transport.NewClient(kind, endpoint, cfg.Token)
}

// Static returns a token function for a fixed token.
func Static(token string) func() string {
	return func() string { return token }
}
