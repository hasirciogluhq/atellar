package shared

import (
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	containerPostgres "github.com/hasirciogluhq/atellar/internal/modules/containers/infrasturcture/repositories"
	nodePostgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
)

type Repositories struct {
	Nodes      *nodePostgres.NodeRepository
	Containers *containerPostgres.ContainerRepository
}

type Infrastructure struct {
	Repositories Repositories
}

func LoadInfrastructure(database *Database) *Infrastructure {
	queries := db_generated.New(database.PgxConn)
	repositories := loadRepositories(queries)

	return &Infrastructure{
		Repositories: *repositories,
	}
}

func loadRepositories(queries *db_generated.Queries) *Repositories {
	return &Repositories{
		Nodes:      nodePostgres.NewNodeRepository(queries),
		Containers: containerPostgres.NewContainerRepository(queries),
	}
}
