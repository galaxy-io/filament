package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies all pending migrations. It is safe to run on every process
// start; goose tracks applied versions in its own goose_db_version table.
// The provider is instance-scoped, so it never collides with the postgres
// store's migrator in the same process.
func Migrate(db *sql.DB) error {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("datastore/sqlite: migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, dir)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("datastore/sqlite: migrate: %w", err)
	}
	return nil
}
