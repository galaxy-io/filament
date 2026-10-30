package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/internal/modules/tracker"
)

// lockLocalRuns keeps separate CLI processes from recovering a live run.
// The lock is held through execution and embedded-server shutdown.
func (a *cliApp) lockLocalRuns() (func(), error) {
	if err := os.MkdirAll(a.stateDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create local state directory: %w", err)
	}
	file, err := os.OpenFile(a.dbPath()+".run.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open local run lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another local run is active")
		}
		return nil, fmt.Errorf("lock local runs: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func (a *cliApp) recoverLocalRuns(ctx context.Context) error {
	store, err := sqlite.Open(a.dbPath())
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	return tracker.RecoverInterrupted(ctx, store)
}
