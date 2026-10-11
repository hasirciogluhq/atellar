//go:build linux

package overlay

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// NewDataplane returns the iproute2 based implementation.
func NewDataplane() Dataplane { return iproute2{} }

type iproute2 struct{}

var errNotFound = errors.New("not found")

func run(name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "Cannot find device") ||
			strings.Contains(msg, "No such") {
			return nil, fmt.Errorf("%s %s: %s: %w", name, strings.Join(args, " "), msg, errNotFound)
		}
		return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func ip(args ...string) ([]byte, error) { return run("ip", args...) }

func ignoreNotFound(err error) error {
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}

func (iproute2) EnsureBridge(b Bridge) error {
	if _, err := ip("link", "show", b.Name); err != nil {
		if _, err := ip("link", "add", b.Name, "type", "bridge"); err != nil {
			return err
		}
	}
	if _, err := ip("link", "set", b.Name, "mtu", strconv.Itoa(b.MTU)); err != nil {
		return err
	}
	// A bridge without ports has no carrier, which marks routes through it
	// linkdown. A dummy port keeps it up before the first container starts.
	dummy := carrierName(b.Name)
	if _, err := ip("link", "show", dummy); err != nil {
		if _, err := ip("link", "add", dummy, "mtu", strconv.Itoa(b.MTU), "type", "dummy"); err != nil {
			return err
		}
	}
	if _, err := ip("link", "set", dummy, "master", b.Name, "up"); err != nil {
		return err
	}
	if err := ensureAddr(b.Name, b.Addr.String()); err != nil {
		return err
	}
	_, err := ip("link", "set", b.Name, "up")
	return err
}

func (iproute2) EnsureVXLAN(v VXLAN) error {
	if _, err := ip("link", "show", v.Name); err != nil {
		if _, err := ip("link", "add", v.Name, "type", "vxlan",
			"id", strconv.Itoa(v.VNI),
			"local", v.Underlay.String(),
			"dstport", strconv.Itoa(v.Port),
			"nolearning"); err != nil {
			return err
		}
	}
	if _, err := ip("link", "set", v.Name, "address", v.MAC.String(), "mtu", strconv.Itoa(v.MTU)); err != nil {
		return err
	}
	if err := ensureAddr(v.Name, v.Addr.String()); err != nil {
		return err
	}
	_, err := ip("link", "set", v.Name, "up")
	return err
}

// ensureAddr leaves exactly one IPv4 address on dev.
func ensureAddr(dev, cidr string) error {
	out, err := ip("-j", "-4", "addr", "show", "dev", dev)
	if err != nil {
		return err
	}
	var links []struct {
		AddrInfo []struct {
			Local     string `json:"local"`
			PrefixLen int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if err := unmarshalRows(out, &links); err != nil {
		return fmt.Errorf("parse addresses of %s: %w", dev, err)
	}
	found := false
	for _, link := range links {
		for _, a := range link.AddrInfo {
			have := a.Local + "/" + strconv.Itoa(a.PrefixLen)
			if have == cidr {
				found = true
				continue
			}
			if _, err := ip("addr", "del", have, "dev", dev); err != nil {
				return err
			}
		}
	}
	if found {
		return nil
	}
	_, err = ip("addr", "add", cidr, "dev", dev)
	return err
}

func carrierName(bridge string) string {
	name := bridge + "-c"
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}

func (iproute2) ListRoutes(dev string) ([]Route, error) {
	out, err := ip("-j", "-4", "route", "show", "dev", dev)
	if err != nil {
		return nil, err
	}
	return parseRoutes(out)
}

func (iproute2) ReplaceRoute(dev string, r Route) error {
	_, err := ip("route", "replace", r.Dst.String(), "via", r.Via.String(), "dev", dev, "onlink")
	return err
}

func (iproute2) DeleteRoute(dev string, r Route) error {
	_, err := ip("route", "del", r.Dst.String(), "via", r.Via.String(), "dev", dev)
	return ignoreNotFound(err)
}

func (iproute2) ListNeighbors(dev string) ([]Neighbor, error) {
	out, err := ip("-j", "-4", "neigh", "show", "dev", dev)
	if err != nil {
		return nil, err
	}
	return parseNeighbors(out)
}

func (iproute2) ReplaceNeighbor(dev string, n Neighbor) error {
	_, err := ip("neigh", "replace", n.IP.String(), "lladdr", n.MAC.String(), "dev", dev, "nud", "permanent")
	return err
}

func (iproute2) DeleteNeighbor(dev string, n Neighbor) error {
	_, err := ip("neigh", "del", n.IP.String(), "dev", dev)
	return ignoreNotFound(err)
}

func (iproute2) ListFDB(dev string) ([]FDBEntry, error) {
	out, err := run("bridge", "-j", "fdb", "show", "dev", dev)
	if err != nil {
		return nil, err
	}
	return parseFDB(out)
}

func (iproute2) ReplaceFDB(dev string, e FDBEntry) error {
	_, err := run("bridge", "fdb", "replace", e.MAC.String(), "dev", dev, "dst", e.Dst.String(), "self", "permanent")
	return err
}

func (iproute2) DeleteFDB(dev string, e FDBEntry) error {
	_, err := run("bridge", "fdb", "del", e.MAC.String(), "dev", dev, "dst", e.Dst.String(), "self")
	return ignoreNotFound(err)
}
