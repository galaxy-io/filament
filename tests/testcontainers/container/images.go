// Package container provides functions for starting and stopping ephemeral
// backing-service containers outside of a test context. State is persisted to
// ~/.config/gx/containers.json so names survive across CLI invocations.
package container

import (
	"fmt"
	"sync"

	"github.com/galaxy-io/filament/tests/testcontainers/internal/envfile"
)

// imageDefaults are used when docker/.env is absent or missing a key.
var imageDefaults = map[string]string{
	"POSTGRES_IMAGE":     "postgres:16.15-alpine",
	"NATS_IMAGE":         "nats:2.14.6-alpine",
	"REDIS_IMAGE":        "redis:7.4.11-alpine",
	"MINIO_IMAGE":        "minio/minio:RELEASE.2025-07-23T15-54-02Z",
	"TRINO_IMAGE":        "trinodb/trino:476",
	"ICEBERG_REST_IMAGE": "apache/iceberg-rest-fixture:1.10.1",
}

// Image returns the pinned image reference for a docker/.env key (e.g.
// "POSTGRES_IMAGE"). Walks up from the working directory to the nearest go.mod
// to find docker/.env — the same file testcontainers and compose both read.
// Falls back to imageDefaults when docker/.env is absent or missing the key.
func Image(key string) (string, error) {
	if v, ok := envVars()[key]; ok {
		return v, nil
	}
	if v, ok := imageDefaults[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("no image for %q and no default — pass --image explicitly", key)
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
		return map[string]string{}
	}
	return envMap
}
