package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hasirciogluhq/atellar/internal/agent"
	"github.com/hasirciogluhq/atellar/internal/agent/config"
	"github.com/hasirciogluhq/atellar/internal/client/controlplane"
)

const (
	DefaultAgentBinPath = "/usr/local/bin/atellar-agent"
	SystemdUnitPath     = "/etc/systemd/system/atellar-agent.service"
)

// --- init (register node + write config) ---

type InitOptions struct {
	Token             string
	ControlPlaneURL   string
	NodeName          string
	PublicIP          string
	PrivateIP         string
	ContainerdSock    string
	HeartbeatInterval string
	ConfigPath        string
}

func Init(ctx context.Context, opts InitOptions) (*config.Config, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("join token is required")
	}

	controlPlaneURL := opts.ControlPlaneURL
	if controlPlaneURL == "" {
		controlPlaneURL = "http://localhost:8080"
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.SystemConfigPath
	}

	containerdSock := opts.ContainerdSock
	if containerdSock == "" {
		containerdSock = "/run/containerd/containerd.sock"
	}

	heartbeatInterval := opts.HeartbeatInterval
	if heartbeatInterval == "" {
		heartbeatInterval = "5s"
	}

	client := controlplane.NewClient(controlPlaneURL)
	result, err := client.Register(ctx, opts.Token, controlplane.RegisterRequest{
		Name:           opts.NodeName,
		PublicIP:       opts.PublicIP,
		PrivateIP:      opts.PrivateIP,
		AgentVersion:   agent.Version,
		ContainerdSock: containerdSock,
	})
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		ControlPlaneURL:   controlPlaneURL,
		NodeID:            result.Node.ID,
		NodeName:          result.Node.Name,
		OverlayIP:         result.Node.OverlayIP.String(),
		OverlaySubnet:     result.Node.OverlaySubnet,
		NodeAPIKey:        result.NodeAPIKey,
		APIKeyExpiresAt:   result.APIKeyExpiresAt,
		ContainerdSock:    containerdSock,
		HeartbeatInterval: heartbeatInterval,
	}

	if err := config.Save(configPath, cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// --- install (systemd) ---

type InstallOptions struct {
	AgentBinSource string
	AgentBinTarget string
	ConfigPath     string
}

type InstallResult struct {
	AgentBinPath string
	ConfigPath   string
	UnitPath     string
}

func Install(opts InstallOptions) (*InstallResult, error) {
	if opts.AgentBinSource == "" {
		return nil, fmt.Errorf("agent binary source path is required")
	}

	agentBinTarget := opts.AgentBinTarget
	if agentBinTarget == "" {
		agentBinTarget = DefaultAgentBinPath
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.SystemConfigPath
	}

	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("agent config not found at %s (run `atelctl agent init` first)", configPath)
	}

	if err := copyFile(opts.AgentBinSource, agentBinTarget, 0o755); err != nil {
		return nil, fmt.Errorf("install agent binary: %w", err)
	}

	unitContents := systemdUnit(agentBinTarget)
	if err := os.WriteFile(SystemdUnitPath, []byte(unitContents), 0o644); err != nil {
		return nil, fmt.Errorf("write systemd unit: %w", err)
	}

	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "atellar-agent.service"},
		{"restart", "atellar-agent.service"},
	} {
		if err := runSystemctl(args...); err != nil {
			return nil, err
		}
	}

	return &InstallResult{
		AgentBinPath: agentBinTarget,
		ConfigPath:   configPath,
		UnitPath:     SystemdUnitPath,
	}, nil
}

// --- renew-key ---

type RenewKeyOptions struct {
	ControlPlaneURL string
	NodeAPIKey      string
	ConfigPath      string
	UpdateConfig    bool
}

type RenewKeyResult struct {
	NodeAPIKey      string
	APIKeyExpiresAt string
	ConfigPath      string
}

func RenewKey(ctx context.Context, opts RenewKeyOptions) (*RenewKeyResult, error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = config.SystemConfigPath
	}

	controlPlaneURL := opts.ControlPlaneURL
	apiKey := opts.NodeAPIKey

	if opts.UpdateConfig || (controlPlaneURL == "" && apiKey == "") {
		cfg, err := config.Load(configPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		if controlPlaneURL == "" {
			controlPlaneURL = cfg.ControlPlaneURL
		}
		if apiKey == "" {
			apiKey = cfg.NodeAPIKey
		}
	}

	if controlPlaneURL == "" {
		return nil, fmt.Errorf("control plane url is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("node api key is required")
	}

	client := controlplane.NewClient(controlPlaneURL)
	renewed, err := client.RenewNodeAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	if opts.UpdateConfig || opts.ConfigPath != "" {
		if err := config.UpdateNodeAPIKey(configPath, renewed.NodeAPIKey, renewed.APIKeyExpiresAt); err != nil {
			return nil, fmt.Errorf("update config: %w", err)
		}
	}

	return &RenewKeyResult{
		NodeAPIKey:      renewed.NodeAPIKey,
		APIKeyExpiresAt: renewed.APIKeyExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		ConfigPath:      configPath,
	}, nil
}

func systemdUnit(agentBinPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Atellar Node Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, agentBinPath)
}

func copyFile(sourcePath, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v failed: %s: %w", args, string(output), err)
	}
	return nil
}
