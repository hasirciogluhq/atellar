//go:build linux

package netns

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

const maxIfaceNameLen = 15

type Config struct {
	ContainerID       string
	OverlayIP         string
	NodeOverlayIP     string
	NodeOverlaySubnet string
	BridgeName        string
}

func Setup(cfg Config) error {
	if cfg.BridgeName == "" {
		cfg.BridgeName = "atellar0"
	}

	gateway := normalizeIPv4(firstNonEmpty(cfg.NodeOverlayIP, gatewayFromBridge(cfg.BridgeName)))
	if gateway == "" {
		return fmt.Errorf("node overlay gateway ip is not configured on %s", cfg.BridgeName)
	}

	containerIP := normalizeIPv4(cfg.OverlayIP)
	if containerIP == "" {
		return fmt.Errorf("container overlay ip is required")
	}

	prefixLen := subnetPrefixLen(cfg.NodeOverlaySubnet)
	addr := fmt.Sprintf("%s/%d", containerIP, prefixLen)

	if err := ensureIPv4Forward(); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
	}

	// Clean partial state from a previous failed attempt.
	Teardown(cfg.ContainerID)

	ns := cfg.ContainerID
	hostVeth, ctrVeth := vethPairNames(cfg.ContainerID)

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

	if out, err := runNs(ns, "ip", "addr", "add", addr, "dev", ctrVeth); err != nil {
		if !strings.Contains(out, "File exists") {
			return fmt.Errorf("addr add: %s: %w", out, err)
		}
	}
	if out, err := runNs(ns, "ip", "link", "set", ctrVeth, "up"); err != nil {
		return fmt.Errorf("ctr veth up: %s: %w", out, err)
	}

	// /32 needs onlink; /24 (same bridge L2 segment) does not.
	routeArgs := []string{"ip", "route", "add", "default", "via", gateway, "dev", ctrVeth}
	if prefixLen == 32 {
		routeArgs = append(routeArgs, "onlink")
	}
	if out, err := runNs(ns, routeArgs...); err != nil {
		if !strings.Contains(out, "File exists") {
			return fmt.Errorf("default route: %s: %w", out, err)
		}
	}

	return nil
}

func Teardown(containerID string) {
	ns := containerID
	hostVeth, _ := vethPairNames(containerID)
	_ = runIgnore("ip", "link", "del", hostVeth)
	_ = runIgnore("ip", "netns", "del", ns)
}

func NetnsPath(containerID string) string {
	return containerID
}

func gatewayFromBridge(bridge string) string {
	out, err := exec.Command("ip", "-4", "addr", "show", "dev", bridge).CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "inet ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip, _, err := net.ParseCIDR(fields[1])
		if err != nil {
			continue
		}
		if v := normalizeIPv4(ip.String()); v != "" {
			return v
		}
	}
	return ""
}

func subnetPrefixLen(cidr string) int {
	if cidr == "" {
		return 32
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 32
	}
	ones, _ := ipNet.Mask.Size()
	if ones <= 0 || ones > 32 {
		return 32
	}
	return ones
}

func normalizeIPv4(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	v4 := parsed.To4()
	if v4 == nil {
		return ""
	}
	return v4.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ensureIPv4Forward() error {
	const path = "/proc/sys/net/ipv4/ip_forward"
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) == "1" {
		return nil
	}
	return os.WriteFile(path, []byte("1\n"), 0o644)
}

// vethPairNames returns Linux ifnames (max 15 chars) derived from container ID.
func vethPairNames(containerID string) (host, peer string) {
	sum := sha256.Sum256([]byte(containerID))
	// vh/vp prefix + 12 hex chars = 14 chars each
	host = "vh" + hex.EncodeToString(sum[:6])
	peer = "vp" + hex.EncodeToString(sum[6:12])
	if len(host) > maxIfaceNameLen || len(peer) > maxIfaceNameLen {
		panic("veth name longer than IFNAMSIZ")
	}
	return host, peer
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
