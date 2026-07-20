// Command control-plane runs Filament's orchestration loop: it executes or
// dispatches requested runs per DISPATCH_MODE and folds worker-emitted facts
// back into Postgres through tracker.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/k8sdispatch"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/secret"

	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		log.Fatal(err)
	}
}

// dispatchModule selects the execution environment for requested runs:
// kubernetes launches one worker Job per run; inproc executes runs inside
// this process through the engine module.
func dispatchModule() (module.Module, error) {
	switch mode := os.Getenv("DISPATCH_MODE"); mode {
	case "", "kubernetes":
		return k8sdispatch.NewFromEnv(), nil
	case "inproc":
		return engine.New(), nil
	default:
		return nil, fmt.Errorf("unknown DISPATCH_MODE %q (kubernetes|inproc)", mode)
	}
}

func run(ctx context.Context) error {
	persistenceDSN := os.Getenv("PERSISTENCE_DSN")
	if persistenceDSN == "" {
		return errors.New("PERSISTENCE_DSN is required")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return errors.New("NATS_URL is required")
	}

	pool, err := ctlpg.NewPool(ctx, persistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)
	secrets, err := secret.FromEnv(ctx, store)
	if err != nil {
		return err
	}

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
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
	busOpts := []natsbus.Option{}
	if stream := os.Getenv("NATS_STREAM"); stream != "" {
		busOpts = append(busOpts, natsbus.WithStream(stream))
	}
	if subjects := os.Getenv("NATS_SUBJECTS"); subjects != "" {
		busOpts = append(busOpts, natsbus.WithSubjects(subjects))
	}
	bus, err := natsbus.New(natsURL, events.Codec, busOpts...)
	if err != nil {
		return err
	}
	defer func() { _ = bus.Close() }()

	dispatch, err := dispatchModule()
	if err != nil {
		return err
	}
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Secrets: secrets, Sources: registry.DefaultSources, Sinks: registry.DefaultSinks},
		tracker.New(),
		dispatch,
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
