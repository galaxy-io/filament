// Package metricsstore selects the MetricsService backend.
package metricsstore

import (
	"context"
	"fmt"
	"os"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/datastore/postgres/metrics"
)

// FromEnv selects the metrics backend per METRICSSTORE_PROVIDER. The default
// (and "postgres") answers from the datastore's own pool — the metrics store
// reads the same runs table server/control-plane persist to. A datastore with
// no metrics support returns nil, leaving MetricsService unimplemented. A
// separate metrics database (e.g. ClickHouse) becomes a new arm here with its
// own DSN, never a topology change.
func FromEnv(_ context.Context, store filament.DataStore) (filament.MetricsStore, error) {
	switch provider := os.Getenv("METRICSSTORE_PROVIDER"); provider {
	case "", "postgres", "postgresql":
		if pg, ok := store.(*postgres.Store); ok {
			return metrics.New(pg.Pool()), nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown METRICSSTORE_PROVIDER %q (postgres)", provider)
	}
}
