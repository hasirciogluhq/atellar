//go:build linux

package overlay

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

type linuxLinkManager struct{}

func newLinkManager() linkManager {
	return &linuxLinkManager{}
}

func (m *linuxLinkManager) EnsureBridge(name string) error {
	if err := m.run("link", "show", name); err != nil {
		if err := m.run("link", "add", name, "type", "bridge"); err != nil {
			return fmt.Errorf("create bridge %s: %w", name, err)
		}
	}

	return m.SetLinkUp(name)
}

func (m *linuxLinkManager) SetLinkUp(name string) error {
	return m.run("link", "set", name, "up")
}

func (m *linuxLinkManager) EnsureAddress(link string, addr *net.IPNet) error {
	if addr == nil {
		return fmt.Errorf("address is required")
	}

	current, err := m.showAddresses(link)
	if err != nil {
		return err
	}

	target := addr.String()
	for _, existing := range current {
		if existing == target {
			return nil
		}
	}

	for _, existing := range current {
		_ = m.run("addr", "del", existing, "dev", link)
	}

	if err := m.run("addr", "add", target, "dev", link); err != nil {
		return fmt.Errorf("assign %s to %s: %w", target, link, err)
	}

	return nil
}

func (m *linuxLinkManager) EnsureRoute(spec RouteSpec) error {
	if spec.Destination == nil || spec.Via == nil {
		return fmt.Errorf("invalid route spec")
	}

	args := []string{
		"route", "replace", spec.Destination.String(),
		"via", spec.Via.String(),
		"dev", spec.Device,
		"onlink",
	}
	return m.run(args...)
}

func (m *linuxLinkManager) DeleteRoute(spec RouteSpec) error {
	if spec.Destination == nil {
		return nil
	}

	args := []string{"route", "del", spec.Destination.String(), "dev", spec.Device}
	if spec.Via != nil {
		args = append(args, "via", spec.Via.String())
	}

	if err := m.run(args...); err != nil {
		if strings.Contains(err.Error(), "No such process") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "Cannot find") {
			return nil
		}
		return err
	}

	return nil
}

func (m *linuxLinkManager) ListRoutes(device string) ([]RouteSpec, error) {
	output, err := exec.Command("ip", "-4", "route", "show", "dev", device).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list routes: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	routes := make([]RouteSpec, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		spec, ok := parseIPRouteLine(line, device)
		if ok {
			routes = append(routes, spec)
		}
	}

	return routes, nil
}

func (m *linuxLinkManager) showAddresses(link string) ([]string, error) {
	output, err := exec.Command("ip", "-4", "addr", "show", "dev", link).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(string(output), "\n")
	addrs := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "inet ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		addrs = append(addrs, fields[1])
	}

	return addrs, nil
}

func parseIPRouteLine(line, device string) (RouteSpec, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return RouteSpec{}, false
	}

	_, dst, err := net.ParseCIDR(fields[0])
	if err != nil {
		ip := net.ParseIP(fields[0])
		if ip == nil {
			return RouteSpec{}, false
		}

		dst = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
	}

	spec := RouteSpec{Destination: dst, Device: device}
	for i := 0; i < len(fields)-1; i++ {
		switch fields[i] {
		case "via":
			spec.Via = net.ParseIP(fields[i+1])
		}
	}

	if spec.Via == nil {
		return RouteSpec{}, false
	}

	return spec, true
}

func (m *linuxLinkManager) run(args ...string) error {
	cmd := exec.Command("ip", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("ip %s: %s", strings.Join(args, " "), message)
	}

	return nil
}
