package overlaynet

import "net"

type linkManager interface {
	EnsureBridge(name string) error
	SetLinkUp(name string) error
	EnsureAddress(link string, addr *net.IPNet) error
	EnsureRoute(spec RouteSpec) error
	DeleteRoute(spec RouteSpec) error
	ListRoutes(device string) ([]RouteSpec, error)
}
