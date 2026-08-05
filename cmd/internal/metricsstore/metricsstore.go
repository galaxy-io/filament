// Package metricsstore selects the MetricsService backend.
package metricsstore

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/galaxy-io/filament"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	metricspg "github.com/galaxy-io/filament/metricsstore/postgres"
)

// FromEnv selects the metrics store per METRICSSTORE_PROVIDER; postgres is
// the default (and only) provider. It reuses PERSISTENCE_DSN — the metrics
// store reads the same runs table server/control-plane persist to, not a
// separate database.
func FromEnv(ctx context.Context) (filament.MetricsStore, error) {
	switch provider := os.Getenv("METRICSSTORE_PROVIDER"); provider {
	case "", "postgres", "postgresql":
		dsn := os.Getenv("PERSISTENCE_DSN")
		if dsn == "" {
			return nil, errors.New("PERSISTENCE_DSN is required")
		}
		pool, err := ctlpg.NewPool(ctx, dsn)
		if err != nil {
			return nil, err
		}
		return metricspg.New(pool), nil
	default:
		return nil, fmt.Errorf("unknown METRICSSTORE_PROVIDER %q (postgres)", provider)
	}
}
