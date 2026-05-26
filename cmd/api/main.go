package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"

	http_routes "github.com/hasirciogluhq/atellar/cmd/api/routes"
	"github.com/hasirciogluhq/atellar/cmd/api/shared"
	"github.com/hasirciogluhq/atellar/internal/config"
	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	"github.com/hasirciogluhq/migrator"
)

func NewDatabase(config *config.APIConfig) *shared.Database {
	return setupDatabaseConnections(config)
}

func setupDatabaseConnections(config *config.APIConfig) *shared.Database {
	pgxConn, err := pgx.Connect(context.Background(), config.DatabaseURL)
	if err != nil {
		panic(err)
	}

	sqlDb, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		panic(err)
	}

	return &shared.Database{PgxConn: pgxConn, SqlDb: sqlDb}
}

func setupMigrations(database *shared.Database, config *config.APIConfig) *migrator.Migrator {
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

	_ = db_generated.New(database.PgxConn)

	infra := shared.LoadInfrastructure(database)

	router := gin.Default()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	http_routes.RegisterRoutes(router.Group("/api"), infra)

	err := http.ListenAndServe(":"+config.Port, router)
	if err != nil {
		panic(err)
	}
}
