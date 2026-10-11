// Package host performs the machine level steps of the install commands: files under
// /etc and systemd units. Paths and the systemctl runner are fields so tests
// can redirect them.
package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Host struct {
	// Root prefixes every path; empty means "/".
	Root string
	// Systemctl runs systemctl; nil uses the real binary.
	Systemctl func(args ...string) error
}

const (
	// Binary is the single atellar executable; both units run it.
	Binary    = "/usr/local/bin/atellar"
	APIUnit   = "atellar-server.service"
	AgentUnit = "atellar-agent.service"
	unitDir   = "/etc/systemd/system"
	LogDir    = "/var/log/atellar"
)

// Path maps an absolute path into Root.
func (h Host) Path(p string) string { return filepath.Join(h.Root, p) }

func (h Host) systemctl(args ...string) error {
	if h.Systemctl != nil {
		return h.Systemctl(args...)
	}
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// WriteFile creates parent directories and writes data with mode.
func (h Host) WriteFile(path string, data []byte, mode os.FileMode) error {
	full := h.Path(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, mode); err != nil {
		return err
	}
	return os.Chmod(full, mode)
}

// RequireFile fails with a hint when path does not exist.
func (h Host) RequireFile(path, hint string) error {
	if _, err := os.Stat(h.Path(path)); err != nil {
		return fmt.Errorf("%s not found: %s", path, hint)
	}
	return nil
}

// InstallUnit writes a unit, reloads systemd and enables it; start also
// (re)starts it.
func (h Host) InstallUnit(name, content string, start bool) error {
	if err := h.WriteFile(unitDir+"/"+name, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	if err := h.systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := h.systemctl("enable", name); err != nil {
		return err
	}
	if start {
		return h.systemctl("restart", name)
	}
	return nil
}

// UnitInstalled reports whether the unit file exists.
func (h Host) UnitInstalled(name string) bool {
	_, err := os.Stat(h.Path(unitDir + "/" + name))
	return err == nil
}

// Restart restarts a unit.
func (h Host) Restart(name string) error { return h.systemctl("restart", name) }

// APIUnitFile runs `atellar server run` with its environment file.
func APIUnitFile(envPath string) string {
	return `[Unit]
Description=Atellar API server
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=` + envPath + `
ExecStart=` + Binary + ` server run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
}

// AgentUnitFile runs `atellar agent run` after containerd.
func AgentUnitFile() string {
	return `[Unit]
Description=Atellar node agent
After=network-online.target containerd.service
Wants=network-online.target
Requires=containerd.service

[Service]
Type=simple
ExecStart=` + Binary + ` agent run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
}
