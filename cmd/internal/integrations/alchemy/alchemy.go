// Package alchemy builds the alchemy client from ALCHEMY_* variables.
package alchemy

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/integrations/alchemy"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/transform"
)

// Config is what a binary needs for alchemy steps; Functions is nil, not a nil client, when disabled.
type Config struct {
	Client        *alchemy.Client
	Functions     transform.FunctionService
	FunctionBatch pipeline.FunctionBatch
}

// FromEnv reads GALAXY_ALCHEMY_SERVER_URL, else GALAXY_ALCHEMY_URL (neither disables alchemy), ALCHEMY_BATCH_MAX_BYTES and ALCHEMY_MAX_IN_FLIGHT; only AUTH_PROVIDER=proxy forwards the tenant.
func FromEnv(store filament.DataStore) (Config, error) {
	url := cmp.Or(os.Getenv("GALAXY_ALCHEMY_SERVER_URL"), os.Getenv("GALAXY_ALCHEMY_URL"))
	if url == "" {
		return Config{}, nil
	}
	batch, err := batchFromEnv()
	if err != nil {
		return Config{}, err
	}
	cfg := alchemy.Config{URL: url, HTTPClient: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}}
	if os.Getenv("AUTH_PROVIDER") == "proxy" {
		dir, ok := store.(filament.TenantDirectory)
		if !ok {
			return Config{}, fmt.Errorf("alchemy: datastore %q cannot map tenants to provider ids", store.Name())
		}
		cfg.Credentials, cfg.Tenants = alchemy.ProxyCredentials{Service: "filament"}, dir
	}
	client, err := alchemy.New(cfg)
	if err != nil {
		return Config{}, err
	}
	return Config{Client: client, Functions: client, FunctionBatch: batch}, nil
}

func batchFromEnv() (pipeline.FunctionBatch, error) {
	var batch pipeline.FunctionBatch
	if v := os.Getenv("ALCHEMY_BATCH_MAX_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 || n > alchemy.DefaultMaxPayloadBytes {
			return batch, fmt.Errorf("alchemy: ALCHEMY_BATCH_MAX_BYTES must be a byte count from 1 to %d, got %q", alchemy.DefaultMaxPayloadBytes, v)
		}
		batch.MaxBytes = n
	}
	if v := os.Getenv("ALCHEMY_MAX_IN_FLIGHT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return batch, fmt.Errorf("alchemy: ALCHEMY_MAX_IN_FLIGHT must be a positive integer, got %q", v)
		}
		batch.MaxInFlight = n
	}
	return batch, nil
}
