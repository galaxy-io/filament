// Package app is the public composition root for a Filament ingestion service.
// It wires the event plane (bus + datastore), tracker, engine, orchestrator,
// and scheduler on a Host and serves the ConnectRPC API, resolving
// source/sink providers at runtime from the process-wide default registry.
// WithNotifier adds webhook delivery.
//
// A deployable binary is therefore just: blank-import the connectors it ships
// (which self-register via init()) and call Run.
//
//	import (
//	    _ "github.com/galaxy-io/filament/connectors/postgres"
//	    "github.com/galaxy-io/filament/app"
//	)
//	func main() { log.Fatal(app.Run(context.Background())) }
//
// The event bus, datastore, and provider registries default to the in-process
// bus, in-memory store, and registry.Default{Sources,Sinks}. Override any of
// them with the With* options:
//
//	bus, _ := nats.New(os.Getenv("NATS_URL"), codec)
//	app.Run(ctx, app.WithBus(bus), app.WithDataStore(pg))
package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/notifier"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/scheduler"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
)

// Config is the resolved composition for a run. Zero value is invalid; build it
// with newConfig + options.
type Config struct {
	Bus     eventbus.Bus
	Store   filament.DataStore
	Metrics filament.MetricsStore
	Secrets filament.Secrets
	Sources filament.SourceRegistry
	Sinks   filament.SinkRegistry
	UI      http.Handler
	Log     filament.Logger

	notifierEnabled bool
}

// Option mutates a Config. Options passed to Run override the defaults.
type Option func(*Config)

// WithBus sets the event-plane transport (default: inproc.New()).
func WithBus(b eventbus.Bus) Option { return func(c *Config) { c.Bus = b } }

// WithDataStore sets the run/checkpoint store (default: sqlite.NewMemory()).
func WithDataStore(s filament.DataStore) Option { return func(c *Config) { c.Store = s } }

// WithMetricsStore sets the run metrics query backend (default: none —
// MetricsService answers Unimplemented).
func WithMetricsStore(ms filament.MetricsStore) Option { return func(c *Config) { c.Metrics = ms } }

// WithSources overrides the source registry (default: registry.DefaultSources).
func WithSources(s filament.SourceRegistry) Option { return func(c *Config) { c.Sources = s } }

// WithSinks overrides the sink registry (default: registry.DefaultSinks).
func WithSinks(s filament.SinkRegistry) Option { return func(c *Config) { c.Sinks = s } }

// WithSecrets sets the secrets provider used to resolve secret refs (default: none).
func WithSecrets(s filament.Secrets) Option { return func(c *Config) { c.Secrets = s } }

// WithNotifier enables webhook delivery using the configured datastore and secrets.
func WithNotifier() Option { return func(c *Config) { c.notifierEnabled = true } }

// WithUI mounts a handler for the web UI at "/" (default: none). The ui
// package provides one: app.WithUI(ui.Handler()). ConnectRPC routes take
// precedence; everything else falls through to the UI handler.
func WithUI(h http.Handler) Option { return func(c *Config) { c.UI = h } }

// WithLogger sets the structured logger used by the API and runtime modules.
func WithLogger(log filament.Logger) Option { return func(c *Config) { c.Log = log } }

// newConfig applies opts, then fills what they left unset. Defaults are
// built only when needed so an overridden store is never opened and leaked.
func newConfig(opts ...Option) Config {
	c := Config{Sources: registry.DefaultSources, Sinks: registry.DefaultSinks}
	for _, o := range opts {
		o(&c)
	}
	if c.Bus == nil {
		c.Bus = inproc.New()
	}
	if c.Store == nil {
		c.Store = sqlite.NewMemory()
	}
	return c
}

// Run mounts the ingestion service and serves the ConnectRPC API until ctx is
// cancelled. With no options it uses the in-process bus, in-memory store, and
// registry.Default{Sources,Sinks}, which connectors populate from init(). The
// listen address comes from INGESTION_ADDR (default ":8080").
func Run(ctx context.Context, opts ...Option) error {
	cfg := newConfig(opts...)
	mux, mounted, cleanup, err := compose(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	for _, name := range mounted {
		fmt.Println("mounted:", name)
	}

	addr := os.Getenv("INGESTION_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if cfg.UI != nil {
		fmt.Println("ui:", "http://localhost"+addr)
	}
	fmt.Println("connectrpc:", "http://localhost"+addr)
	return serve(ctx, ln, mux)
}

// Serve mounts the same stack as Run on a caller-owned listener and serves it
// until ctx is cancelled. It prints nothing.
func Serve(ctx context.Context, ln net.Listener, opts ...Option) error {
	mux, _, cleanup, err := compose(ctx, newConfig(opts...))
	if err != nil {
		return err
	}
	defer cleanup()
	return serve(ctx, ln, mux)
}

// compose mounts the modules and API onto a mux. cleanup closes the host,
// bus, and store.
func compose(ctx context.Context, cfg Config) (mux *http.ServeMux, mounted []string, cleanup func(), err error) {
	orch := orchestrator.New()
	api := server.New(cfg.Sources, cfg.Sinks, cfg.Store, orch, cfg.Bus,
		server.WithSecrets(cfg.Secrets), server.WithMetricsStore(cfg.Metrics), server.WithLogger(cfg.Log))
	scheduleStore, ok := cfg.Store.(filament.ScheduleStore)
	if !ok {
		return nil, nil, nil, fmt.Errorf("datastore %q does not support schedules", cfg.Store.Name())
	}
	sched := scheduler.New(scheduleStore)
	deps := module.Deps{Bus: cfg.Bus, DataStore: cfg.Store, Secrets: cfg.Secrets, Sources: cfg.Sources, Sinks: cfg.Sinks, Log: cfg.Log}
	modules := []module.Module{tracker.New(), engine.New(), orch, sched}
	if cfg.notifierEnabled {
		modules = append(modules, notifier.New())
	}
	mods, err := module.MountAll(ctx, deps, modules...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("mount: %w", err)
	}
	h := host.New(cfg.Bus)
	ctx, cancel := context.WithCancel(ctx)
	cleanup = func() {
		cancel()
		_ = h.Close()
		if c, ok := cfg.Bus.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := cfg.Store.(io.Closer); ok {
			_ = c.Close()
		}
	}
	if err := h.Run(ctx, mods...); err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("run: %w", err)
	}
	sched.Start(ctx)
	mux = http.NewServeMux()
	api.Mount(mux)
	if cfg.UI != nil {
		mux.Handle("/", cfg.UI)
	}
	return mux, h.Mounted(), cleanup, nil
}

// serve runs mux on ln until ctx is cancelled, then shuts down gracefully.
func serve(ctx context.Context, ln net.Listener, mux *http.ServeMux) error {
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// Handlers (long-lived tails) see the cancel so Shutdown does not wait on them.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := srv.Shutdown(shutdownCtx)
		<-errc
		return err
	}
}
