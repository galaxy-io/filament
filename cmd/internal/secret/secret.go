// Package secret selects the filament.Secrets provider.
package secret

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret/aws"
	"github.com/galaxy-io/filament/secret/postgres"
	"github.com/galaxy-io/filament/secret/sqlite"
)

// FromEnv selects the provider per SECRET_PROVIDER. Unset, secrets live in
// the datastore: the provider is chosen by the store's native handle, so it
// follows PERSISTENCE_PROVIDER. aws-secrets-manager uses the default AWS
// config chain and prefixes refs with SECRETS_PREFIX when set.
func FromEnv(ctx context.Context, store filament.DataStore) (filament.Secrets, error) {
	switch provider := os.Getenv("SECRET_PROVIDER"); provider {
	case "":
		switch s := store.(type) {
		case interface{ Pool() *pgxpool.Pool }:
			return postgres.NewFromEnv(s.Pool())
		case interface{ DB() *sql.DB }:
			return sqlite.NewFromEnv(s.DB())
		}
		return nil, fmt.Errorf("secret: datastore %q has no secrets provider", store.Name())
	case "aws-secrets-manager":
		var opts []aws.Option
		if prefix := os.Getenv("SECRETS_PREFIX"); prefix != "" {
			opts = append(opts, aws.WithPrefix(prefix))
		}
		return aws.NewFromConfig(ctx, opts...)
	default:
		return nil, fmt.Errorf("secret: unknown SECRET_PROVIDER %q (unset for the datastore, aws-secrets-manager)", provider)
	}
}
