//go:build integration || e2e

package testcontainers

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	tc "github.com/testcontainers/testcontainers-go"
)

// providerType is explicit because macOS Podman sockets end in -api.sock,
// which testcontainers-go v0.43's default detection does not recognize.
func providerType(t testing.TB) tc.ProviderType {
	t.Helper()
	cfg := tc.ReadConfig().Config
	if cfg.RyukDisabled {
		t.Fatal("Filament shared test containers require Ryuk; remove TESTCONTAINERS_RYUK_DISABLED or ryuk.disabled")
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" && cfg.TestcontainersHost != "" && cfg.TestcontainersHost != host {
		t.Fatal("tc.host in ~/.testcontainers.properties overrides the selected DOCKER_HOST; remove the conflicting tc.host")
	}
	switch os.Getenv("CONTAINER_ENGINE") {
	case "":
		return tc.ProviderDefault
	case "docker":
		return tc.ProviderDocker
	case "podman":
		if os.Getenv("DOCKER_HOST") == "" {
			t.Fatal("Podman tests need DOCKER_HOST; run through just or scripts/container.sh exec")
		}
		return tc.ProviderPodman
	default:
		t.Fatal("CONTAINER_ENGINE must be docker or podman")
		return tc.ProviderDefault
	}
}

// newNetwork applies the same provider to networks as to containers.
// network.New currently has no provider option and can start Ryuk using the
// wrong default network before the first container is created.
func newNetwork(t testing.TB, ctx context.Context) *tc.DockerNetwork {
	t.Helper()
	provider, err := providerType(t).GetProvider()
	if err != nil {
		t.Fatalf("network provider: %v", err)
	}
	defer provider.Close()
	//nolint:staticcheck // v0.43 network.New cannot select a provider.
	nw, err := provider.CreateNetwork(ctx, tc.NetworkRequest{
		Name: uuid.NewString(), Driver: "bridge", Labels: tc.GenericLabels(),
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	return nw.(*tc.DockerNetwork)
}
