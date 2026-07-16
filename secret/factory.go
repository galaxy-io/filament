// Package secret selects the filament.Secrets provider.
package secret

import (
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret/postgres"
)

// FromEnv selects the provider per SECRET_PROVIDER; postgres is the default
// and currently the only option. Datastore-backed providers assert the
// accessor for their store's native handle.
func FromEnv(store filament.DataStore) (filament.Secrets, error) {
	switch provider := os.Getenv("SECRET_PROVIDER"); provider {
	case "", "postgres":
		pg, ok := store.(interface{ Pool() *pgxpool.Pool })
		if !ok {
			return nil, errors.New("secret: SECRET_PROVIDER=postgres requires postgresql persistence")
		}
		return postgres.NewFromEnv(pg.Pool())
	default:
		return nil, fmt.Errorf("secret: unknown SECRET_PROVIDER %q (postgres)", provider)
	}
}
