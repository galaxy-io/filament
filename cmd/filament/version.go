package main

import (
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/version"
)

func (a *cliApp) runVersionCommand() error {
	_, err := fmt.Fprintf(a.stdout, "filament %s\n", version.String())
	return err
}
