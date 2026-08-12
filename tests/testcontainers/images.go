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

// imageDefaults back every key, so a clone with no docker/.env still runs the
// suite. Mirrors container.imageDefaults, which the gx CLI uses for the same
// reason. docker/.env overrides these; it stays the place to pin an exact
// version once for both compose and testcontainers.
var imageDefaults = map[string]string{
	"POSTGRES_IMAGE":     "postgres:latest",
	"NATS_IMAGE":         "nats:latest",
	"REDIS_IMAGE":        "redis:latest",
	"MINIO_IMAGE":        "minio/minio:latest",
	"TRINO_IMAGE":        "trinodb/trino:latest",
	"ICEBERG_REST_IMAGE": "apache/iceberg-rest-fixture:latest",
	"K3S_IMAGE":          "rancher/k3s:v1.31.2-k3s1",
}

// Image returns the pinned image reference for a docker/.env key (e.g.
// "POSTGRES_IMAGE"), falling back to imageDefaults. Reading the same file
// compose uses keeps testcontainers and the dev stack on identical versions —
// bump a tag once in docker/.env.
func Image(t testing.TB, key string) string {
	t.Helper()
	if v, ok := envVars()[key]; ok {
		return v
	}
	if v, ok := imageDefaults[key]; ok {
		return v
	}
	t.Fatalf("no image for %q — add it to docker/.env or imageDefaults", key)
	return ""
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
	// A missing or unreadable docker/.env is not an error: it means "use the
	// defaults". Panicking here took out all thirteen suite tests on any clone
	// that had never created the file.
	if envErr != nil {
		return map[string]string{}
	}
	return envMap
}
