//go:build integration

package persistence_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// TestPostgresPersistenceContracts makes the production datastore and secret
// provider contracts mandatory in the Docker suite. Those package tests accept
// a DSN for local development, but otherwise skip; this wrapper supplies a real
// ephemeral server and rejects any nested skip so CI cannot silently lose the
// persistence coverage again.
func TestPostgresPersistenceContracts(t *testing.T) {
	pg := testcontainers.Postgres(t, testcontainers.WithDatabase("filament_contracts"))
	root := repositoryRoot(t)
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBinary, "test", "-v", "-count=1", "-p=1", "./datastore/postgres", "./secret/postgres")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOWORK=off",
		"FILAMENT_TEST_POSTGRES_DSN="+pg.DSN(),
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("persistence contract tests timed out:\n%s", output)
	}
	if err != nil {
		t.Fatalf("persistence contract tests: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "--- SKIP") {
		t.Fatalf("persistence contract unexpectedly skipped a test:\n%s", output)
	}
}

func repositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
