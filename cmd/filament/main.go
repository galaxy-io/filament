// Command filament manages Filament connections, pipelines, and runs against
// a deployed server or the local engine, filament-standalone.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
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
