package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/version"
)

func (a *cliApp) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintf(a.stdout, "filament %s\n", version.String())
			return err
		},
	}
}
