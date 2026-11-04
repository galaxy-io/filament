package main

import (
	"path/filepath"
	"testing"
)

func TestLocalRunLockPreventsConcurrentRecovery(t *testing.T) {
	app := &cliApp{configPath: filepath.Join(t.TempDir(), "filament.yaml")}
	unlock, err := app.lockLocalRuns()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.lockLocalRuns(); err == nil {
		t.Fatal("second local run acquired the lock")
	}
	unlock()
	unlocked, err := app.lockLocalRuns()
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	unlocked()
}
