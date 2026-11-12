package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/galaxy-io/filament/cmd/internal/cli/engine"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/version"
)

// stateDir holds the local engine's durable state beside the context
// registry: filament.db, secret.key, and the applied-document marker.
// Harness-built apps have no registry and keep it beside the document.
func (a *cliApp) stateDir() string {
	if a.contextPath == "" {
		return filepath.Dir(a.configPath)
	}
	return filepath.Dir(a.contextPath)
}

func (a *cliApp) dbPath() string     { return filepath.Join(a.stateDir(), "filament.db") }
func (a *cliApp) markerPath() string { return filepath.Join(a.stateDir(), "filament.db.applied") }

// engine binds this version's standalone binary to the state directory,
// downloading it on first use.
func (a *cliApp) engine(ctx context.Context) (*engine.Engine, error) {
	binary, err := engine.Locate(ctx, version.Version, a.statusWriter())
	if err != nil {
		return nil, err
	}
	return &engine.Engine{Binary: binary, StateDir: a.stateDir()}, nil
}

// openLocal attaches to the local engine: the detached one up started for
// this state directory, or one started for this command. The document is
// applied into it before anything else talks to it.
func (a *cliApp) openLocal(ctx context.Context) (*localtarget.Target, func(), error) {
	local, err := a.engine(ctx)
	if err != nil {
		return nil, nil, err
	}
	endpoint, stop := "", func() {}
	if state, running := a.readUpState(); running {
		endpoint = "http://" + state.Addr
	} else {
		process, err := local.Start(ctx, "", false)
		if err != nil {
			return nil, nil, err
		}
		endpoint, stop = process.Endpoint, func() { _ = process.Stop() }
	}
	target, err := a.applyLocal(ctx, local, endpoint)
	if err != nil {
		stop()
		return nil, nil, err
	}
	return target, stop, nil
}

// applyLocal targets the engine at endpoint and applies the document into it.
func (a *cliApp) applyLocal(ctx context.Context, local *engine.Engine, endpoint string) (*localtarget.Target, error) {
	target := localtarget.NewTarget(
		localtarget.Store{Path: a.configPath},
		remotetarget.NewTarget(remotetarget.Options{Endpoint: endpoint}),
		local,
	)
	if err := target.ApplyIfChanged(ctx, a.markerPath()); err != nil {
		return nil, fmt.Errorf("apply %s: %w", a.configPath, err)
	}
	return target, nil
}
