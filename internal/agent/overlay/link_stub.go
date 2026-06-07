//go:build !linux

package overlay

import (
	"fmt"
	"log"
	"net"
)

type stubLinkManager struct{}

func newLinkManager() linkManager {
	return &stubLinkManager{}
}

func (m *stubLinkManager) EnsureBridge(name string) error {
	log.Printf("overlay stub: ensure bridge %s", name)
	return nil
}

func (m *stubLinkManager) SetLinkUp(name string) error {
	log.Printf("overlay stub: set link up %s", name)
	return nil
}

func (m *stubLinkManager) EnsureAddress(link string, addr *net.IPNet) error {
	log.Printf("overlay stub: ensure address %s on %s", addr, link)
	return nil
}

func (m *stubLinkManager) EnsureRoute(spec RouteSpec) error {
	log.Printf("overlay stub: ensure route %s via %s dev %s", spec.Destination, spec.Via, spec.Device)
	return nil
}

func (m *stubLinkManager) DeleteRoute(spec RouteSpec) error {
	log.Printf("overlay stub: delete route %s dev %s", spec.Destination, spec.Device)
	return nil
}

func (m *stubLinkManager) ListRoutes(device string) ([]RouteSpec, error) {
	return nil, fmt.Errorf("overlay stub: list routes unsupported on this platform")
}
