// Package agentconfig is the node agent's configuration file,
// /etc/atellar/agent.json. `atellar node join` writes it; `atellar agent run`
// reads it and rewrites it when it renews its API key.
package agentconfig

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/pki"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

const (
	Dir  = "/etc/atellar"
	Path = Dir + "/agent.json"

	DefaultContainerdSock    = "/run/containerd/containerd.sock"
	DefaultHeartbeatInterval = 5 * time.Second
	DefaultLogDir            = "/var/log/atellar"
)

type Config struct {
	ControlPlane client.ControlPlane `json:"control_plane"`
	// Transport is "grpc" (default) or "http".
	Transport string `json:"transport,omitempty"`
	// CACert is the cluster CA (PEM) the agent verifies the API server
	// with. `node join` fetches it and checks it against --ca-cert-hash.
	CACert string `json:"ca_cert"`

	NodeID          string    `json:"node_id"`
	NodeName        string    `json:"node_name,omitempty"`
	NodeAPIKey      string    `json:"node_api_key"`
	APIKeyExpiresAt time.Time `json:"api_key_expires_at"`

	ContainerdSock    string   `json:"containerd_sock,omitempty"`
	HeartbeatInterval Duration `json:"heartbeat_interval,omitempty"`
	LogDir            string   `json:"log_dir,omitempty"`

	Network Network `json:"network,omitempty"`
}

// Network tunes the overlay; every field has a default in pkg/network/overlay.
type Network struct {
	// UnderlayIP overrides the node's registered private IP as the VXLAN
	// source address.
	UnderlayIP        string   `json:"underlay_ip,omitempty"`
	BridgeName        string   `json:"bridge_name,omitempty"`
	VXLANName         string   `json:"vxlan_name,omitempty"`
	VNI               int      `json:"vni,omitempty"`
	Port              int      `json:"port,omitempty"`
	MTU               int      `json:"mtu,omitempty"`
	ReconcileInterval Duration `json:"reconcile_interval,omitempty"`
}

// Duration is a time.Duration written as a string such as "5s".
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(time.Duration(d).String()) }

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*d = 0
		return nil
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// SetDefaults fills optional fields.
func (c *Config) SetDefaults() {
	if c.Transport == "" {
		c.Transport = string(transport.GRPC)
	}
	if c.ContainerdSock == "" {
		c.ContainerdSock = DefaultContainerdSock
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = Duration(DefaultHeartbeatInterval)
	}
	if c.LogDir == "" {
		c.LogDir = DefaultLogDir
	}
}

func (c *Config) Validate() error {
	if err := c.ControlPlane.Validate(); err != nil {
		return err
	}
	if _, err := transport.ParseKind(c.Transport); err != nil {
		return err
	}
	switch {
	case c.CACert == "":
		return errors.New("ca_cert is missing; run `atellar node trust-ca --ca-cert-hash sha256:...`")
	case c.NodeID == "":
		return errors.New("node_id is required")
	case c.NodeAPIKey == "":
		return errors.New("node_api_key is required")
	case c.APIKeyExpiresAt.IsZero():
		return errors.New("api_key_expires_at is required")
	}
	return nil
}

// TLS returns the client TLS configuration that trusts the cluster CA.
func (c *Config) TLS() (*tls.Config, error) {
	cfg, err := pki.ClientConfig([]byte(c.CACert))
	if err != nil {
		return nil, fmt.Errorf("ca_cert: %w", err)
	}
	return cfg, nil
}

// Load reads, defaults and validates a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("agent config not found at %s; join the node with `atellar node join`", path)
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the file atomically with mode 0600: it holds the API key.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
