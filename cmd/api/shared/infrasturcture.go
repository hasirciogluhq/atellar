package shared

import (
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	containerPostgres "github.com/hasirciogluhq/atellar/internal/modules/containers/infrasturcture/repositories"
	nodeAuth "github.com/hasirciogluhq/atellar/internal/modules/nodes/application/auth"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
	"github.com/hasirciogluhq/atellar/internal/pkg/authn"
)

type Repositories struct {
	Nodes      *nodePostgres.NodeRepository
	Containers *containerPostgres.ContainerRepository
}

type Infrastructure struct {
	Repositories Repositories
	NodeAuth     authn.Authenticator
}

func LoadInfrastructure(database *Database) *Infrastructure {
	queries := db_generated.New(database.PgxConn)
	repositories := loadRepositories(queries)

	return &Infrastructure{
		Repositories: *repositories,
		NodeAuth:     nodeAuth.NewNodeAuthenticator(repositories.Nodes),
	}
}

func loadRepositories(queries *db_generated.Queries) *Repositories {
	return &Repositories{
		Nodes:      nodePostgres.NewNodeRepository(queries),
		Containers: containerPostgres.NewContainerRepository(queries),
	}
}
