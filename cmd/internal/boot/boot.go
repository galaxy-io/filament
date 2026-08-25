// Package boot resolves the env-driven dependencies every deployed binary
// shares and mounts modules on a host over the event bus.
package boot

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/eventbus"
	"github.com/galaxy-io/filament/cmd/internal/logger"
	"github.com/galaxy-io/filament/cmd/internal/otel"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	bus "github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
)

// Deps are the providers every deployed binary resolves from the environment.
type Deps struct {
	Log     filament.Logger
	Store   filament.DataStore
	Secrets filament.Secrets
	Metrics filament.Metrics
	Tracer  filament.Tracer
}

// FromEnv builds the logger, datastore, secrets, and otel providers. The
// returned close flushes otel and closes the store; call it on the way out.
// The event bus is deliberately separate (Bus) so binaries can start health
// listeners before the connect wait.
func FromEnv(ctx context.Context) (Deps, func(), error) {
	lg := logger.New()
	store, err := persistence.FromEnv(ctx)
	if err != nil {
		return Deps{}, nil, err
	}
	closeStore := func() {
		if c, ok := store.(io.Closer); ok {
			_ = c.Close()
		}
	}
	secrets, err := secret.FromEnv(ctx, store)
	if err != nil {
		closeStore()
		return Deps{}, nil, err
	}
	metrics, tracer, otelShutdown, err := otel.FromEnv(ctx)
	if err != nil {
		closeStore()
		return Deps{}, nil, err
	}
	shutdown := func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(flushCtx)
		closeStore()
	}
	return Deps{Log: lg, Store: store, Secrets: secrets, Metrics: metrics, Tracer: tracer}, shutdown, nil
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
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
		Log:       d.Log,
		Metrics:   d.Metrics,
		Tracer:    d.Tracer,
	}, mods...)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
	}
	h := host.New(b)
	if err := h.Run(ctx, rs...); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("run host: %w", err)
	}
	return h, nil
}
