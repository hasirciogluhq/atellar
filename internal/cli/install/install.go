package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hasirciogluhq/atellar/internal/pkg/agentconfig"
)

const (
	DefaultAgentBinPath = "/usr/local/bin/atellar-agent"
	SystemdUnitPath     = "/etc/systemd/system/atellar-agent.service"
)

type Options struct {
	AgentBinSource string
	AgentBinTarget string
	ConfigPath     string
}

type Result struct {
	AgentBinPath string
	ConfigPath   string
	UnitPath     string
}

func Execute(opts Options) (*Result, error) {
	agentBinSource := opts.AgentBinSource
	if agentBinSource == "" {
		return nil, fmt.Errorf("agent binary source path is required")
	}

	agentBinTarget := opts.AgentBinTarget
	if agentBinTarget == "" {
		agentBinTarget = DefaultAgentBinPath
	}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = agentconfig.SystemConfigPath
	}

	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("agent config not found at %s (run `atellar join` first)", configPath)
	}

	if err := copyFile(agentBinSource, agentBinTarget, 0o755); err != nil {
		return nil, fmt.Errorf("install agent binary: %w", err)
	}

	unitContents := systemdUnit(agentBinTarget)
	if err := os.WriteFile(SystemdUnitPath, []byte(unitContents), 0o644); err != nil {
		return nil, fmt.Errorf("write systemd unit: %w", err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return nil, err
	}

	if err := runSystemctl("enable", "atellar-agent.service"); err != nil {
		return nil, err
	}

	if err := runSystemctl("restart", "atellar-agent.service"); err != nil {
		return nil, err
	}

	return &Result{
		AgentBinPath: agentBinTarget,
		ConfigPath:   configPath,
		UnitPath:     SystemdUnitPath,
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

	if _, err := io.Copy(target, source); err != nil {
		return err
	}

	return nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v failed: %s: %w", args, string(output), err)
	}

	return nil
}
