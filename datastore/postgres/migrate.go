package postgres

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies all pending migrations using the given *sql.DB (a
// "pgx" stdlib connection — see NewSQLDB). Safe to call on every process
// start; goose tracks applied versions in its own goose_db_version table.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("datastore/postgres: set dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("datastore/postgres: migrate: %w", err)
	}
	return nil
}
