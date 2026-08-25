// Command control-plane runs Filament's orchestration loop: it fires due
// schedules, executes or dispatches requested runs per DISPATCH_MODE, and
// folds worker-emitted facts back into Postgres through tracker.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/cmd/internal/dispatch"
	"github.com/galaxy-io/filament/internal/modules/reaper"
	"github.com/galaxy-io/filament/internal/modules/scheduler"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"

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
	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer closeDeps()

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := deps.Store.Ping(pingCtx); err != nil {
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
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthSrv.Shutdown(shutCtx)
	}()

	bus, closeBus, err := boot.Bus()
	if err != nil {
		return err
	}
	defer closeBus()

	dispatcher, err := dispatch.FromEnv()
	if err != nil {
		return err
	}
	scheduleStore, ok := deps.Store.(filament.ScheduleStore)
	if !ok {
		return fmt.Errorf("datastore %q does not support schedules", deps.Store.Name())
	}
	sched := scheduler.New(scheduleStore)
	mods := []module.Module{tracker.New(), dispatcher, sched}
	// The reaper mounts only under kubernetes dispatch: staleness means death
	// only where heartbeats exist, and inproc runs don't emit them. The Alive
	// probe holds kills for workers that are up but silent.
	var reap *reaper.Module
	if alive, ok := dispatcher.(interface {
		Alive(context.Context, filament.RunID) (bool, error)
	}); ok {
		reap = reaper.NewFromEnv(reaper.WithAliveCheck(alive.Alive))
		mods = append(mods, reap)
	}
	h, err := boot.Mount(ctx, deps, bus, mods...)
	if err != nil {
		return err
	}
	defer func() {
		_ = h.Close()
	}()
	sched.Start(ctx)
	if reap != nil {
		reap.Start(ctx)
	}
	for _, name := range h.Mounted() {
		fmt.Println("mounted:", name)
	}

	<-ctx.Done()
	return nil
}
