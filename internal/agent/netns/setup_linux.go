//go:build linux

package netns

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type Config struct {
	ContainerID string
	OverlayIP   string
	BridgeName  string
	GatewayIP   string
}

func Setup(cfg Config) error {
	if cfg.BridgeName == "" {
		cfg.BridgeName = "atellar0"
	}
	if cfg.GatewayIP == "" {
		cfg.GatewayIP = gatewayFromBridge(cfg.BridgeName)
	}

	ns := cfg.ContainerID
	hostVeth := trimVethHost(cfg.ContainerID)
	ctrVeth := trimVethCtr(cfg.ContainerID)

	if out, err := run("ip", "netns", "add", ns); err != nil {
		if !strings.Contains(out, "File exists") {
			return fmt.Errorf("netns add: %s: %w", out, err)
		}
	}

	if out, err := run("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", ctrVeth); err != nil {
		if !strings.Contains(out, "File exists") {
			return fmt.Errorf("veth add: %s: %w", out, err)
		}
	}

	if out, err := run("ip", "link", "set", hostVeth, "master", cfg.BridgeName); err != nil {
		return fmt.Errorf("veth master: %s: %w", out, err)
	}
	if out, err := run("ip", "link", "set", hostVeth, "up"); err != nil {
		return fmt.Errorf("host veth up: %s: %w", out, err)
	}

	if out, err := run("ip", "link", "set", ctrVeth, "netns", ns); err != nil {
		return fmt.Errorf("veth netns: %s: %w", out, err)
	}

	if out, err := runNs(ns, "ip", "addr", "add", cfg.OverlayIP+"/32", "dev", ctrVeth); err != nil {
		return fmt.Errorf("addr add: %s: %w", out, err)
	}
	if out, err := runNs(ns, "ip", "link", "set", ctrVeth, "up"); err != nil {
		return fmt.Errorf("ctr veth up: %s: %w", out, err)
	}
	if out, err := runNs(ns, "ip", "route", "add", "default", "via", cfg.GatewayIP, "dev", ctrVeth); err != nil {
		if !strings.Contains(out, "File exists") {
			return fmt.Errorf("default route: %s: %w", out, err)
		}
	}

	return nil
}

func Teardown(containerID string) {
	ns := containerID
	hostVeth := trimVethHost(containerID)
	_ = runIgnore("ip", "link", "del", hostVeth)
	_ = runIgnore("ip", "netns", "del", ns)
}

func NetnsPath(containerID string) string {
	return containerID
}

func gatewayFromBridge(bridge string) string {
	out, err := exec.Command("ip", "-4", "addr", "show", "dev", bridge).CombinedOutput()
	if err != nil {
		return "10.0.0.1"
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			ip, _, err := net.ParseCIDR(fields[1])
			if err == nil {
				return ip.String()
			}
		}
	}
	return "10.0.0.1"
}

func trimVethHost(id string) string {
	const max = 15
	name := "veth" + strings.ReplaceAll(id, "_", "")
	if len(name) > max {
		return name[:max]
	}
	return name
}

func trimVethCtr(id string) string {
	return trimVethHost(id) + "p"
}

func run(args ...string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func runNs(ns string, args ...string) (string, error) {
	full := append([]string{"ip", "netns", "exec", ns}, args...)
	return run(full...)
}

func runIgnore(args ...string) error {
	_, err := run(args...)
	return err
}
