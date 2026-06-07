package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultFileName  = "agent.json"
	SystemConfigDir  = "/etc/atellar"
	SystemConfigPath = SystemConfigDir + "/" + DefaultFileName
	DefaultGrpcPort  = "9090"
)

type Config struct {
	ControlPlaneURL   string    `json:"control_plane_url"`
	GrpcAddr          string    `json:"grpc_addr,omitempty"`
	NodeID            string    `json:"node_id"`
	NodeName          string    `json:"node_name,omitempty"`
	OverlayIP         string    `json:"overlay_ip,omitempty"`
	OverlaySubnet     string    `json:"overlay_subnet,omitempty"`
	NodeAPIKey        string    `json:"node_api_key"`
	APIKeyExpiresAt   time.Time `json:"api_key_expires_at"`
	ContainerdSock    string    `json:"containerd_sock,omitempty"`
	HeartbeatInterval string    `json:"heartbeat_interval,omitempty"`
	BridgeName        string    `json:"bridge_name,omitempty"`
	ReconcileInterval string    `json:"reconcile_interval,omitempty"`
}

func (c *Config) ResolveGrpcAddr() string {
	if c.GrpcAddr != "" {
		return c.GrpcAddr
	}

	parsed, err := url.Parse(c.ControlPlaneURL)
	if err != nil || parsed.Host == "" {
		return "localhost:" + DefaultGrpcPort
	}

	return parsed.Hostname() + ":" + DefaultGrpcPort
}

func (c *Config) ResolveBridgeName() string {
	if c.BridgeName != "" {
		return c.BridgeName
	}
	return "atellar0"
}

func (c *Config) ResolveReconcileInterval() time.Duration {
	if c.ReconcileInterval == "" {
		return 30 * time.Second
	}

	parsed, err := time.ParseDuration(c.ReconcileInterval)
	if err != nil {
		return 30 * time.Second
	}

	return parsed
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("config not found at %s", path)
		}

		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	var legacy struct {
		NodeToken      string    `json:"node_token"`
		TokenExpiresAt time.Time `json:"token_expires_at"`
	}
	if err := json.Unmarshal(data, &legacy); err == nil {
		if cfg.NodeAPIKey == "" && legacy.NodeToken != "" {
			cfg.NodeAPIKey = legacy.NodeToken
		}
		if cfg.APIKeyExpiresAt.IsZero() && !legacy.TokenExpiresAt.IsZero() {
			cfg.APIKeyExpiresAt = legacy.TokenExpiresAt
		}
	}

	if cfg.ControlPlaneURL == "" {
		return nil, errors.New("control_plane_url is required in config")
	}

	if cfg.NodeID == "" {
		return nil, errors.New("node_id is required in config")
	}

	if cfg.NodeAPIKey == "" {
		return nil, errors.New("node_api_key is required in config")
	}

	if cfg.APIKeyExpiresAt.IsZero() {
		return nil, errors.New("api_key_expires_at is required in config")
	}

	if cfg.HeartbeatInterval == "" {
		cfg.HeartbeatInterval = "5s"
	}

	if cfg.ContainerdSock == "" {
		cfg.ContainerdSock = "/run/containerd/containerd.sock"
	}

	return &cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func UpdateNodeAPIKey(path string, apiKey string, expiresAt time.Time) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}

	cfg.NodeAPIKey = apiKey
	cfg.APIKeyExpiresAt = expiresAt

	return Save(path, *cfg)
}
