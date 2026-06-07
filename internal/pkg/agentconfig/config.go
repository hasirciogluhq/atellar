package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultFileName = "agent.json"
	SystemConfigDir = "/etc/atellar"
	SystemConfigPath = SystemConfigDir + "/" + DefaultFileName
)

type Config struct {
	ControlPlaneURL   string `json:"control_plane_url"`
	NodeID            string `json:"node_id"`
	NodeName          string `json:"node_name,omitempty"`
	ContainerdSock    string `json:"containerd_sock,omitempty"`
	HeartbeatInterval string `json:"heartbeat_interval,omitempty"`
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

	if cfg.ControlPlaneURL == "" {
		return nil, errors.New("control_plane_url is required in config")
	}

	if cfg.NodeID == "" {
		return nil, errors.New("node_id is required in config")
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
