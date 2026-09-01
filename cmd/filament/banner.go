package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/galaxy-io/filament/cmd/internal/cli/style"
	"github.com/galaxy-io/filament/cmd/internal/version"
)

// banner is the one-line identity printed above every command.
func banner() string {
	return fmt.Sprintf("Filament CLI %s (Go %s)", version.Version, strings.TrimPrefix(runtime.Version(), "go"))
}

// showBanner skips commands whose output is consumed by other programs.
func showBanner(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "version", "completion", "__complete", "__completeNoDesc", "--version", "-v":
		return false
	}
	return true
}

func (a *cliApp) printBanner(args []string) error {
	if !showBanner(args) {
		return nil
	}
	_, err := fmt.Fprintln(a.statusWriter(), style.New(a.statusWriter()).Label(banner()))
	return err
}
