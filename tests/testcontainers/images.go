//go:build integration

// Package testcontainers provides helpers for spinning up ephemeral backing
// services in integration tests. All helpers register t.Cleanup so containers
// are terminated automatically and tests never leak resources.
//
// Gated behind //go:build integration so everyday `go test ./...` stays fast
// and Docker-free.
package testcontainers

import (
	"sync"
	"testing"

	"github.com/galaxy-io/filament/tests/testcontainers/internal/envfile"
)

// Image returns the pinned image reference for a docker/.env key (e.g.
// "POSTGRES_IMAGE"). Reading the same file compose uses keeps testcontainers
// and the dev stack on identical versions — bump a tag once in docker/.env.
func Image(t testing.TB, key string) string {
	t.Helper()
	v, ok := envVars()[key]
	if !ok {
		t.Fatalf("docker/.env has no %q — add it there so dev and test share one version", key)
	}
	return v
}

var (
	envOnce sync.Once
	envMap  map[string]string
	envErr  error
)

func envVars() map[string]string {
	envOnce.Do(func() {
		path, err := envfile.FindEnv()
		if err != nil {
			envErr = err
			return
		}
		envMap, envErr = envfile.Parse(path)
	})
	if envErr != nil {
		panic(envErr)
	}
	return envMap
}
