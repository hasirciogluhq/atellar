package shared

import (
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	"github.com/hasirciogluhq/atellar/internal/config"
	"github.com/hasirciogluhq/atellar/internal/grpc/agentregistry"
	containerports "github.com/hasirciogluhq/atellar/internal/modules/containers/ports"
	containerPostgres "github.com/hasirciogluhq/atellar/internal/modules/containers/infrasturcture/repositories"
	nodeAuth "github.com/hasirciogluhq/atellar/internal/modules/nodes/application/auth"
	nodeservices "github.com/hasirciogluhq/atellar/internal/modules/nodes/application/services"
	nodeports "github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
)

type Repositories struct {
	Nodes      *nodePostgres.NodeRepository
	Containers *containerPostgres.ContainerRepository
}

type Infrastructure struct {
	Repositories          Repositories
	NodeAuth              authn.Authenticator
	AgentRegistry         *agentregistry.Registry
	NodePeerNotifier      nodeports.PeerNotifier
	ContainerPeerNotifier containerports.PeerNotifier
	OverlayProvisioner    nodeports.OverlayProvisioner
}

func LoadInfrastructure(database *Database, apiConfig *config.APIConfig) (*Infrastructure, error) {
	queries := db_generated.New(database.PgxConn)
	repositories := loadRepositories(queries)

	agentRegistry := agentregistry.NewRegistry()
	peerNotifier := agentregistry.NewPeerNotifier(agentRegistry)

	overlayProvisioner, err := nodeservices.NewOverlayProvisioner(
		repositories.Nodes,
		repositories.Containers,
		apiConfig.ClusterOverlayCIDR,
		apiConfig.NodeSubnetPrefixLen,
	)
	if err != nil {
		return nil, err
	}

	return &Infrastructure{
		Repositories:          *repositories,
		NodeAuth:              nodeAuth.NewNodeAuthenticator(repositories.Nodes),
		AgentRegistry:         agentRegistry,
		NodePeerNotifier:      peerNotifier,
		ContainerPeerNotifier: peerNotifier,
		OverlayProvisioner:    overlayProvisioner,
	}, nil
}

func loadRepositories(queries *db_generated.Queries) *Repositories {
	return &Repositories{
		Nodes:      nodePostgres.NewNodeRepository(queries),
		Containers: containerPostgres.NewContainerRepository(queries),
	}
}
