// Package api defines the Atellar API once, independent of any transport.
// pkg/apiserver implements it; pkg/transport/* carry it over the network.
package api

import (
	"context"

	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

// Interface is the full Atellar API surface.
type Interface interface {
	Nodes() NodeInterface
	Containers() ContainerInterface
	JoinTokens() JoinTokenInterface
}

// NodeInterface manages cluster nodes. Methods named Self* act on the node
// that owns the caller's credential (agents only).
type NodeInterface interface {
	List(ctx context.Context) ([]v1.Node, error)
	// Get accepts a node ID ("node_...") or a node name.
	Get(ctx context.Context, ref string) (*v1.Node, error)
	// Watch streams the current nodes as ADDED events, one BOOKMARK, then
	// every change until ctx is done.
	Watch(ctx context.Context) (watch.Interface[v1.Node], error)
	Evict(ctx context.Context, ref string) (*v1.Node, error)
	ListOverlayIPs(ctx context.Context, ref string, freeOnly bool) ([]v1.OverlayIP, error)

	// Register needs no credential; the join token in the request is the proof.
	Register(ctx context.Context, req v1.RegisterNodeRequest) (*v1.RegisterNodeResponse, error)

	SelfHeartbeat(ctx context.Context) error
	SelfReportHardware(ctx context.Context, hw v1.NodeHardware) error
	SelfRenewAPIKey(ctx context.Context) (*v1.NodeAPIKey, error)
}

// ContainerInterface manages containers.
type ContainerInterface interface {
	List(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error)
	Get(ctx context.Context, id string) (*v1.Container, error)
	// Watch streams matching containers as ADDED events, one BOOKMARK, then
	// every change until ctx is done.
	Watch(ctx context.Context, opts v1.ContainerListOptions) (watch.Interface[v1.Container], error)
	Create(ctx context.Context, req v1.CreateContainerRequest) (*v1.Container, error)
	Delete(ctx context.Context, id string) (*v1.Container, error)
	ListEvents(ctx context.Context, id string) ([]v1.ContainerEvent, error)

	// Agent-only: the container must belong to the caller's node.
	ReportRuntime(ctx context.Context, id string, report v1.ContainerRuntimeReport) (*v1.Container, error)
	AllocateOverlayIP(ctx context.Context, id string) (*v1.Container, error)
}

// JoinTokenInterface manages node join tokens.
type JoinTokenInterface interface {
	List(ctx context.Context) ([]v1.JoinToken, error)
	Create(ctx context.Context, req v1.CreateJoinTokenRequest) (*v1.JoinToken, error)
}
