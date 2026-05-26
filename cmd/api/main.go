package main

import (
	"context"
	"database/sql"
	"errors"
	"os"

	_ "github.com/lib/pq"

	db_generated "github.com/hasirciogluhq/atellar/internal/db/generated"
	"github.com/hasirciogluhq/migrator"
	"github.com/jackc/pgx/v5"
)

func main() {
	connString := "postgresql://postgres:1234@localhost:5432/atellar_cp"

	os.Setenv("MIGRATIONS_PATH", "./internal/db/migrations")

	// ready migrator.
	sqlDb, err := sql.Open("postgres", connString)
	if err != nil {
		panic(err)
	}

	mgr := migrator.NewWithOptions(sqlDb, migrator.Options{
		DatabaseURL:    connString,
		MigrationsPath: "./internal/db/migrations",
	})
	if mgr == nil {
		panic(errors.New("Migrator is nil"))
	}

	err = mgr.Migrate(context.Background())
	if err != nil {
		panic(err)
	}

	// we can connect for production useCase
	// Because migration is ended
	pgxConn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		panic(err)
	}

	_ = db_generated.New(pgxConn)
}
