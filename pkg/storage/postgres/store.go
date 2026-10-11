// Package postgres implements storage.Interface on PostgreSQL via sqlc.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/postgres/db"
)

// Store is a storage.Interface backed by a pgx pool.
type Store struct {
	pool *pgxpool.Pool // nil inside a transaction
	q    *db.Queries
}

var _ storage.Interface = (*Store)(nil)

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: db.New(pool)}
}

func (s *Store) Tx(ctx context.Context, fn func(storage.Interface) error) error {
	if s.pool == nil {
		// Already in a transaction: join it.
		return fn(s)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{q: s.q.WithTx(tx)})
	})
}

// notFound maps pgx.ErrNoRows to storage.ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

// wrap maps driver errors to storage sentinels and adds op context.
func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	err = notFound(err)
	if errors.Is(err, storage.ErrNotFound) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case uniqueViolation:
			return fmt.Errorf("%s: %w: %s", op, storage.ErrConflict, pgErr.Detail)
		case foreignKeyViolation:
			return fmt.Errorf("%s: %w: %s", op, storage.ErrNotFound, pgErr.Detail)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}
