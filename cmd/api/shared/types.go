package shared

import (
	"database/sql"

	"github.com/jackc/pgx/v5"
)

type Database struct {
	PgxConn *pgx.Conn
	SqlDb   *sql.DB
}
