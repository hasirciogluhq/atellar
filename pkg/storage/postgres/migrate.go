package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hasirciogluhq/migrator"
	_ "github.com/lib/pq" // database/sql driver used by the migrator
)

// Migrate applies the SQL files in migrationsPath.
func Migrate(ctx context.Context, databaseURL, migrationsPath string) error {
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	mgr := migrator.NewWithOptions(sqlDB, migrator.Options{
		DatabaseURL:    databaseURL,
		MigrationsPath: migrationsPath,
	})
	if mgr == nil {
		return errors.New("migrator is nil")
	}
	return mgr.Migrate(ctx)
}
