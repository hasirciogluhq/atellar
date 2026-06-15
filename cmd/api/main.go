package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"

	"github.com/hasirciogluhq/atellar/internal/config"
	bootstrap "github.com/hasirciogluhq/atellar/internal/controlplane/bootstrap"
	http_routes "github.com/hasirciogluhq/atellar/internal/controlplane/transport/http/routes"
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	grpcserver "github.com/hasirciogluhq/atellar/internal/grpc/server"
	"github.com/hasirciogluhq/migrator"
)

func NewDatabase(config *config.APIConfig) *bootstrap.Database {
	return setupDatabaseConnections(config)
}

func setupDatabaseConnections(config *config.APIConfig) *bootstrap.Database {
	pool, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		panic(err)
	}

	sqlDb, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		panic(err)
	}

	return &bootstrap.Database{Pool: pool, SqlDb: sqlDb}
}

func setupMigrations(database *bootstrap.Database, config *config.APIConfig) *migrator.Migrator {
	mgr := migrator.NewWithOptions(database.SqlDb, migrator.Options{
		DatabaseURL:    config.DatabaseURL,
		MigrationsPath: config.MigrationsPath,
	})

	if mgr == nil {
		panic(errors.New("Migrator is nil"))
	}

	err := mgr.Migrate(context.Background())
	if err != nil {
		panic(err)
	}

	return mgr
}

func main() {
	if os.Getenv("DATABASE_URL") == "" {
		os.Setenv("DATABASE_URL", "postgresql://postgres:1234@localhost:5432/atellar_cp?sslmode=disable")
	}

	config := config.NewAPIConfig()

	fmt.Println("Database URL: ", config.DatabaseURL)

	database := NewDatabase(config)
	setupMigrations(database, config)

	_ = db_generated.New(database.Pool)

	infra, err := bootstrap.LoadInfrastructure(database, config)
	if err != nil {
		panic(err)
	}

	go func() {
		log.Printf("grpc server listening on :%s", config.GRPCPort)
		if err := grpcserver.ListenAndServe(config.GRPCPort, grpcserver.Deps{
			NodeAuth:              infra.NodeAuth,
			Authz:                 infra.Authz,
			Nodes:                 infra.Repositories.Nodes,
			Containers:            infra.Repositories.Containers,
			AgentRegistry:         infra.AgentRegistry,
			ContainerPeerNotifier: infra.ContainerPeerNotifier,
		}); err != nil {
			panic(err)
		}
	}()

	router := gin.Default()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	http_routes.RegisterRoutes(router.Group("/api"), infra)

	log.Printf("http server listening on :%s", config.Port)
	err = http.ListenAndServe(":"+config.Port, router)
	if err != nil {
		panic(err)
	}
}
