// Package secret selects the filament.Secrets provider.
package secret

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret/aws"
	"github.com/galaxy-io/filament/secret/postgres"
)

// FromEnv selects the provider per SECRET_PROVIDER; postgres is the default.
// Datastore-backed providers assert the accessor for their store's native
// handle; aws-secrets-manager uses the default AWS config chain and prefixes
// refs with SECRETS_PREFIX when set.
func FromEnv(ctx context.Context, store filament.DataStore) (filament.Secrets, error) {
	switch provider := os.Getenv("SECRET_PROVIDER"); provider {
	case "", "postgres":
		pg, ok := store.(interface{ Pool() *pgxpool.Pool })
		if !ok {
			return nil, errors.New("secret: SECRET_PROVIDER=postgres requires postgresql persistence")
		}
		return postgres.NewFromEnv(pg.Pool())
	case "aws-secrets-manager":
		var opts []aws.Option
		if prefix := os.Getenv("SECRETS_PREFIX"); prefix != "" {
			opts = append(opts, aws.WithPrefix(prefix))
		}
		return aws.NewFromConfig(ctx, opts...)
	default:
		return nil, fmt.Errorf("secret: unknown SECRET_PROVIDER %q (postgres, aws-secrets-manager)", provider)
	}
}
