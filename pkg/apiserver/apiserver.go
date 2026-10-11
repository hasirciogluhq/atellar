// Package apiserver is the Atellar control plane logic. Server implements
// api.Interface once; transports in pkg/transport expose it over the network.
package apiserver

import (
	"github.com/hasirciogluhq/atellar/pkg/api"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/containers"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/jointokens"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/nodes"
	"github.com/hasirciogluhq/atellar/pkg/apiserver/registry"
	"github.com/hasirciogluhq/atellar/pkg/auth/authz"
	"github.com/hasirciogluhq/atellar/pkg/events"
	"github.com/hasirciogluhq/atellar/pkg/network/ipam"
	"github.com/hasirciogluhq/atellar/pkg/scheduler"
	"github.com/hasirciogluhq/atellar/pkg/storage"
)

type Config struct {
	Store storage.Interface
	// ClusterCIDR is split into one subnet per node (default 10.0.0.0/8).
	ClusterCIDR string
	// NodeSubnetPrefixLen is the size of each node subnet (default 24).
	NodeSubnetPrefixLen int
	// Authorizer defaults to authz.DefaultPolicy.
	Authorizer *authz.Authorizer
}

type Server struct {
	deps       *registry.Deps
	nodes      *nodes.Registry
	containers *containers.Registry
	joinTokens *jointokens.Registry
}

var _ api.Interface = (*Server)(nil)

func New(cfg Config) (*Server, error) {
	allocator, err := ipam.NewAllocator(cfg.ClusterCIDR, cfg.NodeSubnetPrefixLen)
	if err != nil {
		return nil, err
	}
	authorizer := cfg.Authorizer
	if authorizer == nil {
		authorizer = authz.New(nil)
	}

	deps := &registry.Deps{
		Store:           cfg.Store,
		Authz:           authorizer,
		NodeEvents:      events.NewBroker[v1.Node](),
		ContainerEvents: events.NewBroker[v1.Container](),
	}
	return &Server{
		deps:       deps,
		nodes:      nodes.New(deps, allocator),
		containers: containers.New(deps, scheduler.New(cfg.Store)),
		joinTokens: jointokens.New(deps),
	}, nil
}

func (s *Server) Nodes() api.NodeInterface           { return s.nodes }
func (s *Server) Containers() api.ContainerInterface { return s.containers }
func (s *Server) JoinTokens() api.JoinTokenInterface { return s.joinTokens }

// NodeEvents lets in-process controllers publish node changes.
func (s *Server) NodeEvents() *events.Broker[v1.Node] { return s.deps.NodeEvents }
