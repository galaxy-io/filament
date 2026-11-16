package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain builds the standalone engine the local commands start.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "filament-engine-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binary := filepath.Join(dir, "filament-standalone")
	build := exec.Command("go", "build", "-o", binary, "../standalone")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build standalone:", err)
		os.Exit(1)
	}
	_ = os.Setenv("FILAMENT_ENGINE", binary)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestNonInteractiveConfigurationLifecycle(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "filament.yaml")
	var output bytes.Buffer
	app := cliApp{stdout: &output, stderr: &output, configPath: path}
	run := func(args ...string) {
		t.Helper()
		if err := app.run(context.Background(), args); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
	}

	run("source", "create", "demo", "--source-connector", "sample")
	run("sink", "create", "out", "--sink-connector", "stdout")
	run("pipeline", "create", "copy", "--source", "demo", "--sink", "out", "--resources", "users", "--source-rows", "2", "--sync-mode", "full", "--write-mode", "replace")
	run("source", "discover", "demo")
	run("run", "copy")
	run("config", "validate")
}

func TestNonTerminalWithoutCommandFallsBackToHelp(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	app := cliApp{
		stdin: strings.NewReader(""), stdout: &output, stderr: &output,
		configPath: filepath.Join(t.TempDir(), "filament.yaml"),
	}
	if err := app.run(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Filament configures and runs data pipelines") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRootIncludesUpgradeCommand(t *testing.T) {
	t.Parallel()
	app := cliApp{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	command, _, err := app.rootCommand().Find([]string{"upgrade"})
	if err != nil {
		t.Fatal(err)
	}
	if command.Name() != "upgrade" {
		t.Fatalf("command = %q, want upgrade", command.Name())
	}
}

func TestUnreachableRemoteContextFailsBeforeRendererExecution(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	contextPath := filepath.Join(directory, "contexts.yaml")
	data := []byte("current: deployed\ncontexts:\n  deployed:\n    kind: remote\n    endpoint: https://filament.example.test\n")
	if err := os.WriteFile(contextPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	app := cliApp{
		stdin: strings.NewReader(""), stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{},
		configPath: filepath.Join(directory, "filament.yaml"), contextPath: contextPath,
	}
	err := app.run(context.Background(), []string{"source", "list"})
	if err == nil || !strings.Contains(err.Error(), "cannot reach https://filament.example.test") {
		t.Fatalf("error = %v, want cannot reach https://filament.example.test", err)
	}
}
