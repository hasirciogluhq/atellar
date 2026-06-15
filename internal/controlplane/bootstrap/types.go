package bootstrap

import (
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool  *pgxpool.Pool
	SqlDb *sql.DB
}
