//go:build !linux

package overlaynet

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
	log.Printf("overlaynet stub: ensure bridge %s", name)
	return nil
}

func (m *stubLinkManager) SetLinkUp(name string) error {
	log.Printf("overlaynet stub: set link up %s", name)
	return nil
}

func (m *stubLinkManager) EnsureAddress(link string, addr *net.IPNet) error {
	log.Printf("overlaynet stub: ensure address %s on %s", addr, link)
	return nil
}

func (m *stubLinkManager) EnsureRoute(spec RouteSpec) error {
	log.Printf("overlaynet stub: ensure route %s via %s dev %s", spec.Destination, spec.Via, spec.Device)
	return nil
}

func (m *stubLinkManager) DeleteRoute(spec RouteSpec) error {
	log.Printf("overlaynet stub: delete route %s dev %s", spec.Destination, spec.Device)
	return nil
}

func (m *stubLinkManager) ListRoutes(device string) ([]RouteSpec, error) {
	return nil, fmt.Errorf("overlaynet stub: list routes unsupported on this platform")
}
