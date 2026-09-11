// Command control-plane runs Filament's orchestration loop: it fires due
// schedules, executes or dispatches requested runs per DISPATCH_MODE, and
// folds worker-emitted facts back into Postgres through tracker.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/cmd/internal/dispatch"
	"github.com/galaxy-io/filament/cmd/internal/health"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/internal/modules/notifier"
	"github.com/galaxy-io/filament/internal/modules/reaper"
	"github.com/galaxy-io/filament/internal/modules/scheduler"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	notification "github.com/galaxy-io/filament/internal/notifier"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
	"github.com/galaxy-io/filament/module"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		slog.Error("control-plane exited",
			"event.name", "control_plane.exited",
			"error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer closeDeps()
	log := deps.Log.With(
		filament.Field{Key: "component", Value: "control-plane"},
	)

	healthMux := http.NewServeMux()
	var eventBus eventbus.Bus
	healthState := health.New(2*time.Second,
		deps.Store.Ping,
		func(ctx context.Context) error {
			if checker, ok := eventBus.(eventbus.ReadinessChecker); ok {
				return checker.Ready(ctx)
			}
			return nil
		},
	)
	healthState.Mount(healthMux)
	healthAddr := os.Getenv("HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8081"
	}
	healthSrv := &http.Server{Addr: healthAddr, Handler: healthMux, ReadHeaderTimeout: 10 * time.Second}
	healthErr := make(chan error, 1)
	go func() { healthErr <- healthSrv.ListenAndServe() }()
	defer shutdownHealth(healthSrv, healthState, log)

	eventBus, closeBus, err := boot.Bus()
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
	notify := notifier.New(map[notification.NotificationType]notification.Sender{
		notification.NotificationWebhook: webhook.New(nil),
	})
	mods := []module.Module{tracker.New(), dispatcher, sched, notify}
	// The reaper mounts only under kubernetes dispatch: staleness means death
	// only where heartbeats exist, and inproc runs don't emit them. The
	// workload probe holds kills for workers that are up but silent and for
	// Requested runs that were never dispatched.
	var reap *reaper.Module
	if prober, ok := dispatcher.(interface {
		Workload(context.Context, filament.RunID) (filament.Workload, error)
	}); ok {
		reap = reaper.NewFromEnv(reaper.WithWorkloadProbe(prober.Workload))
		mods = append(mods, reap)
	}
	h, err := boot.Mount(ctx, deps, eventBus, mods...)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
		if err := h.Close(); err != nil {
			log.Warn("control-plane host close failed",
				filament.Field{Key: "event.name", Value: "control_plane.host.close_failed"},
				filament.Field{Key: "error", Value: err.Error()})
		}
	}()
	sched.Start(ctx)
	if reap != nil {
		reap.Start(ctx)
	}
	healthState.MarkStarted()
	mode := os.Getenv("DISPATCH_MODE")
	if mode == "" {
		mode = "kubernetes"
	}
	log.Info("control-plane started",
		filament.Field{Key: "event.name", Value: "control_plane.started"},
		filament.Field{Key: "dispatch_mode", Value: mode},
		filament.Field{Key: "health_address", Value: healthAddr},
		filament.Field{Key: "modules", Value: h.Mounted()},
	)

	select {
	case <-ctx.Done():
		log.Info("control-plane stopping",
			filament.Field{Key: "event.name", Value: "control_plane.stopping"})
		return nil
	case err := <-healthErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("control-plane health server: %w", err)
	}
}

func shutdownHealth(srv *http.Server, state *health.State, log filament.Logger) {
	state.MarkStopping()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("control-plane health server shutdown failed", err,
			filament.Field{Key: "event.name", Value: "control_plane.health.shutdown_failed"})
	}
}
