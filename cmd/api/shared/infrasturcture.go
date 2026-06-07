package shared

import (
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	postgres "github.com/hasirciogluhq/atellar/internal/modules/nodes/infrasturcture/repositories"
)

type Repositories struct {
	Nodes *postgres.NodeRepository
}

type Infrastructure struct {
	Repositories Repositories
}

func LoadInfrastructure(database *Database) *Infrastructure {
	queries := db_generated.New(database.PgxConn)
	repositories := loadRepositories(database, queries)

	return &Infrastructure{
		Repositories: *repositories,
	}
}

func loadRepositories(database *Database, queries *db_generated.Queries) *Repositories {
	return &Repositories{
		Nodes: postgres.NewNodeRepository(queries),
	}
}
