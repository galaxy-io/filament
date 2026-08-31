// Command filament configures and runs Filament locally.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	os.Exit(runMain())
}

func runMain() int {
	path, err := defaultConfigPath()
	if err != nil {
		printCLIError(err)
		return 1
	}
	cli := &cliApp{
		stdin:       os.Stdin,
		stdout:      os.Stdout,
		stderr:      os.Stderr,
		configPath:  path,
		contextPath: filepath.Join(filepath.Dir(path), "contexts.yaml"),
		catalog:     loadCatalog(),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.run(ctx, os.Args[1:]); err != nil {
		printCLIError(err)
		return 1
	}
	return 0
}

func printCLIError(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "filament:", err)
}
