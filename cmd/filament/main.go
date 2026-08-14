// Command filament configures and runs Filament locally.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	path, err := defaultConfigPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "filament:", err)
		os.Exit(1)
	}
	cli := &cliApp{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		configPath: path,
		catalog:    loadCatalog(),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "filament:", err)
		os.Exit(1)
	}
}
