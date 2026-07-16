// Package secret selects the ingestion.Secrets provider.
package secret

import (
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret/postgres"
)

// FromEnv selects the provider per SECRET_PROVIDER; postgres is the default
// and currently the only option.
func FromEnv(pool *pgxpool.Pool) (ingestion.Secrets, error) {
	switch provider := os.Getenv("SECRET_PROVIDER"); provider {
	case "", "postgres":
		return postgres.NewFromEnv(pool)
	default:
		return nil, fmt.Errorf("secret: unknown SECRET_PROVIDER %q (postgres)", provider)
	}
}
