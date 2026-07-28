// Command control-plane runs Filament's orchestration loop: it executes or
// dispatches requested runs per DISPATCH_MODE and folds worker-emitted facts
// back into Postgres through tracker.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/galaxy-io/filament/cmd/internal/dispatch"
	"github.com/galaxy-io/filament/cmd/internal/eventbus"
	"github.com/galaxy-io/filament/cmd/internal/otel"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	store, err := persistence.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := store.(io.Closer); ok {
			_ = c.Close()
		}
	}()
	secrets, err := secret.FromEnv(ctx, store)
	if err != nil {
		return err
	}
	metrics, tracer, otelShutdown, err := otel.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(flushCtx)
	}()

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(pingCtx); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8081"
	}
	healthSrv := &http.Server{Addr: healthAddr, Handler: healthMux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = healthSrv.ListenAndServe() }()
	defer func() { _ = healthSrv.Close() }()
	bus, err := eventbus.FromEnv()
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := any(bus).(io.Closer); ok {
			_ = c.Close()
		}
	}()

	dispatcher, err := dispatch.FromEnv()
	if err != nil {
		return err
	}
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Secrets: secrets, Sources: registry.DefaultSources, Sinks: registry.DefaultSinks, Metrics: metrics, Tracer: tracer},
		tracker.New(),
		dispatcher,
	)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	h := host.New(bus)
	defer func() {
		_ = h.Close()
		if c, ok := any(bus).(io.Closer); ok {
			_ = c.Close()
		}
	}()
	if err := h.Run(ctx, mods...); err != nil {
		return fmt.Errorf("run host: %w", err)
	}
	for _, name := range h.Mounted() {
		fmt.Println("mounted:", name)
	}

	<-ctx.Done()
	return nil
}
