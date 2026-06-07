package server

import (
	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	containerPostgres "github.com/hasirciogluhq/atellar/internal/modules/containers/infrasturcture/repositories"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
)

type Deps struct {
	NodeAuth      authn.Authenticator
	Nodes         *nodePostgres.NodeRepository
	Containers    *containerPostgres.ContainerRepository
	AgentRegistry *agentregistry.Registry
}
