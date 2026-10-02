// Package boot resolves the env-driven dependencies every deployed binary
// shares and mounts modules on a host over the event bus.
package boot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/catalog"
	"github.com/galaxy-io/filament/cmd/internal/eventbus"
	"github.com/galaxy-io/filament/cmd/internal/logger"
	"github.com/galaxy-io/filament/cmd/internal/otel"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	"github.com/galaxy-io/filament/datastore/postgres"
	bus "github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
)

// Deps are the providers every deployed binary resolves from the environment.
type Deps struct {
	// Sources is set by the binaries that carry drivers. FromEnv leaves it
	// nil so a binary that imports boot links none.
	Sources filament.SourceRegistry
	// Catalog answers the scheduler's connector questions. Unset, modules
	// fall back to the registries.
	Catalog filament.Catalog
	Log     filament.Logger
	Store   filament.DataStore
	// StreamStore is the same underlying store exposed through its stream interface.
	StreamStore filament.ContinuousRunStore
	Secrets     filament.Secrets
	Metrics     filament.Metrics
	Tracer      filament.Tracer
}

// FromEnv builds the logger, datastore, secrets, and otel providers. The
// returned close flushes otel and closes the secrets provider and store; call
// it on the way out.
// The event bus is deliberately separate (Bus) so binaries can start health
// listeners before the connect wait.
func FromEnv(ctx context.Context) (Deps, func(), error) {
	lg, metrics, tracer, closeTelemetry, err := Telemetry(ctx)
	if err != nil {
		return Deps{}, nil, err
	}
	store, err := persistence.FromEnv(ctx)
	if err != nil {
		closeTelemetry()
		return Deps{}, nil, err
	}
	var streamStore filament.ContinuousRunStore
	if pg, ok := store.(*postgres.Store); ok {
		pg.ConfigureStreamCodecs(registry.DefaultCodecs)
		streamStore = pg
	}
	closeStore := func() {
		if c, ok := store.(io.Closer); ok {
			_ = c.Close()
		}
	}
	secrets, err := secret.FromEnv(ctx, store)
	if err != nil {
		closeStore()
		closeTelemetry()
		return Deps{}, nil, err
	}
	shutdown := func() {
		closeTelemetry()
		if c, ok := secrets.(io.Closer); ok {
			_ = c.Close()
		}
		closeStore()
	}
	return Deps{Log: lg, Store: store, StreamStore: streamStore, Secrets: secrets, Metrics: metrics, Tracer: tracer}, shutdown, nil
}

// Telemetry builds the logger and otel providers alone, for a process that
// needs no datastore. The returned close flushes otel.
func Telemetry(ctx context.Context) (filament.Logger, filament.Metrics, filament.Tracer, func(), error) {
	lg, err := logger.New()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	metrics, tracer, otelShutdown, err := otel.FromEnv(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	flush := func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(flushCtx)
	}
	return lg, metrics, tracer, flush, nil
}

// RemoteCatalog reaches the catalog at CATALOG_URL, for binaries that
// link no driver.
func RemoteCatalog() (filament.Catalog, error) {
	url := os.Getenv("CATALOG_URL")
	if url == "" {
		return nil, errors.New("CATALOG_URL is required")
	}
	return catalog.Remote(url), nil
}

// Bus connects the event bus. The returned close closes it when closable.
func Bus() (bus.Bus, func(), error) {
	b, err := eventbus.FromEnv()
	if err != nil {
		return nil, nil, err
	}
	return b, func() {
		if c, ok := b.(io.Closer); ok {
			_ = c.Close()
		}
	}, nil
}

// Mount builds module.Deps from d, mounts mods on a host over b, and starts
// them. Caller closes the returned host.
func Mount(ctx context.Context, d Deps, b bus.Bus, mods ...module.Module) (*host.Host, error) {
	rs, err := module.MountAll(ctx, module.Deps{
		Bus:       b,
		DataStore: d.Store,
		Secrets:   d.Secrets,
		Sources:   d.Sources,
		Sinks:     registry.DefaultSinks,
		Catalog:   d.Catalog,
		Log:       d.Log,
		Metrics:   d.Metrics,
		Tracer:    d.Tracer,
	}, mods...)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}
	h := host.New(b, host.WithHandlerError(func(module, pattern, durable string, seq uint64, err error) {
		if d.Log == nil {
			return
		}
		d.Log.Error("event handler failed", err,
			filament.Field{Key: "event.name", Value: "eventbus.handler.failed"},
			filament.Field{Key: "component", Value: "eventbus"},
			filament.Field{Key: "module", Value: module},
			filament.Field{Key: "subject_pattern", Value: pattern},
			filament.Field{Key: "durable", Value: durable},
			filament.Field{Key: "stream_sequence", Value: seq},
		)
	}))
	if err := h.Run(ctx, rs...); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("run host: %w", err)
	}
	return h, nil
}
