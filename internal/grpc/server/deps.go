package server

import (
	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	containerPostgres "github.com/hasirciogluhq/atellar/internal/modules/containers/infrasturcture/repositories"
	containerports "github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
	"github.com/hasirciogluhq/atellar/internal/platform/authz"
)

type Deps struct {
	NodeAuth              authn.Authenticator
	Authz                 *authz.Authorizer
	Nodes                 *nodePostgres.NodeRepository
	Containers            *containerPostgres.ContainerRepository
	AgentRegistry         *agentregistry.Registry
	ContainerPeerNotifier containerports.PeerNotifier
}
