// Package persistence selects the datastore backend.
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/galaxy-io/filament"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
)

// FromEnv selects the datastore per PERSISTENCE_PROVIDER; postgres is the
// default. postgres pools connections from PERSISTENCE_DSN.
func FromEnv(ctx context.Context) (filament.DataStore, error) {
	switch provider := os.Getenv("PERSISTENCE_PROVIDER"); provider {
	case "", "postgres":
		dsn, err := dsnFromEnv()
		if err != nil {
			return nil, err
		}
		pool, err := ctlpg.NewPool(ctx, dsn)
		if err != nil {
			return nil, err
		}
		return ctlpg.New(pool), nil
	default:
		return nil, fmt.Errorf("unknown PERSISTENCE_PROVIDER %q (postgres)", provider)
	}
}

// MigrateFromEnv migrates the selected datastore, waiting for it to become
// reachable first.
func MigrateFromEnv(ctx context.Context) error {
	switch provider := os.Getenv("PERSISTENCE_PROVIDER"); provider {
	case "", "postgres":
		dsn, err := dsnFromEnv()
		if err != nil {
			return err
		}
		db, err := ctlpg.NewSQLDB(dsn)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := waitForDB(ctx, db); err != nil {
			return err
		}
		return ctlpg.Migrate(db)
	default:
		return fmt.Errorf("unknown PERSISTENCE_PROVIDER %q (postgres)", provider)
	}
}

func dsnFromEnv() (string, error) {
	dsn := os.Getenv("PERSISTENCE_DSN")
	if dsn == "" {
		return "", errors.New("PERSISTENCE_DSN is required")
	}
	return dsn, nil
}

// waitForDB pings until the database accepts connections, so the migrate init
// container rides out postgres still starting on a fresh install.
func waitForDB(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database not reachable: %w", err)
		}
		log.Printf("waiting for database: %v", err)
		time.Sleep(2 * time.Second)
	}
}
