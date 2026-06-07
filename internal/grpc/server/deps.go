package server

import (
	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
)

type Deps struct {
	NodeAuth      authn.Authenticator
	Nodes         *nodePostgres.NodeRepository
	AgentRegistry *agentregistry.Registry
}
