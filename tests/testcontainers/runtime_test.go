//go:build integration

package testcontainers

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/moby/moby/client"
	tc "github.com/testcontainers/testcontainers-go"
)

// TestRuntimeCleanup proves shared containers are reaped even when Go cleanup
// cannot run. Each child owns a separate Testcontainers/Ryuk session.
func TestRuntimeCleanup(t *testing.T) {
	providerType(t)
	api, err := tc.NewDockerClientWithOpts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer api.Close()
	for _, mode := range []string{"success", "failure", "interrupt"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			defer cancel()
			idPath := filepath.Join(t.TempDir(), "container-id")
			// Testcontainers derives the session ID from the parent PID. Give each
			// case its own waiting parent so it cannot reuse a terminating Ryuk.
			cmd := exec.CommandContext(ctx, "sh", "-c", `"$@" & wait $!`, "cleanup-parent",
				os.Args[0], "-test.run=^TestRuntimeCleanupChild$", "-test.timeout=90s")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				if errors.Is(err, syscall.ESRCH) {
					return os.ErrProcessDone
				}
				return err
			}
			cmd.Env = append(os.Environ(), "FILAMENT_CLEANUP_CHILD="+mode, "FILAMENT_CLEANUP_ID="+idPath)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			var id []byte
			var childErr error
			exited := false
			for len(id) == 0 {
				id, _ = os.ReadFile(idPath)
				if len(id) != 0 {
					break
				}
				select {
				case childErr = <-done:
					exited = true
					id, _ = os.ReadFile(idPath)
					if len(id) == 0 {
						t.Fatalf("child exited before recording a container: %v", childErr)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(100 * time.Millisecond):
				}
			}
			// Only a fallback if the assertion fails; it cannot hide a Ryuk failure.
			t.Cleanup(func() {
				cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
				defer stop()
				_, _ = api.ContainerRemove(cleanupCtx, string(id), client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
			})
			if mode == "interrupt" {
				if err := cmd.Cancel(); err != nil {
					t.Fatal(err)
				}
			}
			if !exited {
				childErr = <-done
			}
			if mode == "success" && childErr != nil {
				t.Fatalf("child: %v", childErr)
			}
			if mode != "success" && childErr == nil {
				t.Fatal("expected an unsuccessful child exit")
			}
			deadline := time.Now().Add(45 * time.Second)
			for {
				// List avoids Podman's transient inspect error while Ryuk removes a container.
				containers, err := api.ContainerList(ctx, client.ContainerListOptions{
					All: true, Filters: make(client.Filters).Add("id", string(id)),
				})
				if err != nil {
					t.Fatalf("list during cleanup: %v", err)
				}
				if len(containers.Items) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("Ryuk did not remove the shared container within 45s")
				}
				time.Sleep(200 * time.Millisecond)
			}
		})
	}
}

func TestRuntimeCleanupChild(t *testing.T) {
	mode := os.Getenv("FILAMENT_CLEANUP_CHILD")
	if mode == "" {
		t.Skip("only invoked by TestRuntimeCleanup")
	}
	pg := SharedPostgres(t)
	if err := os.WriteFile(os.Getenv("FILAMENT_CLEANUP_ID"), []byte(pg.Container.GetContainerID()), 0o600); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "failure":
		t.Fatal("intentional child failure to exercise Ryuk cleanup")
	case "interrupt":
		<-t.Context().Done()
	}
}
