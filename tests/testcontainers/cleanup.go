//go:build integration || e2e

package testcontainers

import (
	"context"
	"io"
	"testing"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
)

const maxFailureLogBytes = 2 << 20

// cleanupContainer prints bounded container logs when a test fails before it
// terminates the container. Startup and service failures are otherwise very
// difficult to diagnose from CI's test output alone.
func cleanupContainer(t testing.TB, name string, container tc.Container) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if t.Failed() {
			logs, err := container.Logs(ctx)
			if err != nil {
				t.Logf("%s container logs unavailable: %v", name, err)
			} else {
				defer logs.Close()
				body, readErr := io.ReadAll(io.LimitReader(logs, maxFailureLogBytes))
				if readErr != nil {
					t.Logf("read %s container logs: %v", name, readErr)
				}
				if len(body) != 0 {
					t.Logf("%s container logs (up to %d bytes):\n%s", name, maxFailureLogBytes, body)
				}
			}
		}
		if err := tc.TerminateContainer(container); err != nil {
			t.Logf("terminate %s container: %v", name, err)
		}
	})
}
