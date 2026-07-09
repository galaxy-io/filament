// Package app is the public composition root for a Filament ingestion service.
// It wires the event plane (bus + datastore), the tracker, engine, and
// orchestrator modules on a Host) and serves the ConnectRPC API, resolving
// source/sink providers at runtime from the process-wide default registry.
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
	"net/http"
	"os"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
)

// Config is the resolved composition for a run. Zero value is invalid; build it
// with newConfig + options. Each field is an interface so deployments can swap
// transports and stores without touching the wiring below.
type Config struct {
	Bus     eventbus.Bus
	Store   ingestion.DataStore
	Sources ingestion.SourceRegistry
	Sinks   ingestion.SinkRegistry
	UI      http.Handler
}

// Option mutates a Config. Options passed to Run override the defaults.
type Option func(*Config)

// WithBus sets the event-plane transport (default: inproc.New()).
func WithBus(b eventbus.Bus) Option { return func(c *Config) { c.Bus = b } }

// WithDataStore sets the run/checkpoint store (default: memory.New()).
func WithDataStore(s ingestion.DataStore) Option { return func(c *Config) { c.Store = s } }

// WithSources overrides the source registry (default: registry.DefaultSources).
func WithSources(s ingestion.SourceRegistry) Option { return func(c *Config) { c.Sources = s } }

// WithSinks overrides the sink registry (default: registry.DefaultSinks).
func WithSinks(s ingestion.SinkRegistry) Option { return func(c *Config) { c.Sinks = s } }

// WithUI mounts a handler for the web UI at "/" (default: none). The ui
// package provides one: app.WithUI(ui.Handler()). ConnectRPC routes take
// precedence; everything else falls through to the UI handler.
func WithUI(h http.Handler) Option { return func(c *Config) { c.UI = h } }

func newConfig(opts ...Option) Config {
	c := Config{
		Bus:     inproc.New(),
		Store:   memory.New(),
		Sources: registry.DefaultSources,
		Sinks:   registry.DefaultSinks,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// Run mounts the ingestion service and serves the ConnectRPC API until the
// process is signalled. With no options it uses the in-process bus, in-memory
// store, and registry.Default{Sources,Sinks}, which connectors populate from
// init(). The listen address comes from INGESTION_ADDR (default ":8080").
func Run(ctx context.Context, opts ...Option) error {
	cfg := newConfig(opts...)

	orch := orchestrator.New()
	deps := module.Deps{Bus: cfg.Bus, DataStore: cfg.Store, Sources: cfg.Sources, Sinks: cfg.Sinks}
	mods, err := module.MountAll(ctx, deps, tracker.New(), engine.New(), orch)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	h := host.New(cfg.Bus)
	defer func() {
		_ = h.Close()
		if c, ok := cfg.Bus.(io.Closer); ok {
			_ = c.Close()
		}
		if c, ok := cfg.Store.(io.Closer); ok {
			_ = c.Close()
		}
	}()
	if err := h.Run(ctx, mods...); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	for _, name := range h.Mounted() {
		fmt.Println("mounted:", name)
	}

	addr := os.Getenv("INGESTION_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	server.New(cfg.Sources, cfg.Sinks, cfg.Store, orch, cfg.Bus).Mount(mux)
	if cfg.UI != nil {
		mux.Handle("/", cfg.UI)
		fmt.Println("ui:", "http://localhost"+addr)
	}
	fmt.Println("connectrpc:", "http://localhost"+addr)
	return http.ListenAndServe(addr, mux)
}
