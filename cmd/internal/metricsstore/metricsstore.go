// Package metricsstore selects the MetricsService backend.
package metricsstore

import (
	"context"
	"fmt"
	"os"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres"
	pgmetrics "github.com/galaxy-io/filament/datastore/postgres/metrics"
	"github.com/galaxy-io/filament/datastore/sqlite"
	sqlitemetrics "github.com/galaxy-io/filament/datastore/sqlite/metrics"
)

// FromEnv selects the metrics backend per METRICSSTORE_PROVIDER. Unset, the
// metrics store answers from the datastore's own handle — it reads the same
// runs table server/control-plane persist to, whichever backend that is. A
// datastore with no metrics support returns nil, leaving MetricsService
// unimplemented. A separate metrics database (e.g. ClickHouse) becomes a
// named arm here with its own DSN, never a topology change.
func FromEnv(_ context.Context, store filament.DataStore) (filament.MetricsStore, error) {
	switch provider := os.Getenv("METRICSSTORE_PROVIDER"); provider {
	case "":
		switch s := store.(type) {
		case *postgres.Store:
			return pgmetrics.New(s.Pool()), nil
		case *sqlite.Store:
			return sqlitemetrics.New(s.DB()), nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown METRICSSTORE_PROVIDER %q (unset for the datastore)", provider)
	}
}
