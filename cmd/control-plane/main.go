package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	cp_db "github.com/hasirciogluhq/atellar/control-plane/internal/db/generated"
	"github.com/hasirciogluhq/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

	db := cp_db.New(pgxConn)

	perfNow := time.Now()
	db.CreateUser(context.Background(), cp_db.CreateUserParams{
		Name: "Ahmet",
		Bio: pgtype.Text{
			String: "Developer",
			Valid:  true,
		},
	})

	since := time.Since(perfNow)

	users, err := db.GetUsers(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(users)
	fmt.Println(since)
}
