package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/storagetest"
)

// TestConformance needs a disposable database, for example:
//
//	ATELLAR_TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/atellar_test?sslmode=disable
//
// Every table is truncated between subtests.
func TestConformance(t *testing.T) {
	url := os.Getenv("ATELLAR_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ATELLAR_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	if err := Migrate(ctx, url, "migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	storagetest.Run(t, func(t *testing.T) storage.Interface {
		if _, err := pool.Exec(ctx, `TRUNCATE container_events, overlay_ip_pool, containers, node_join_tokens, nodes CASCADE`); err != nil {
			t.Fatal(err)
		}
		return New(pool)
	})
}
