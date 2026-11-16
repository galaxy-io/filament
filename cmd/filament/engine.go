package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/engine"
	"github.com/galaxy-io/filament/version"
)

func (a *cliApp) engineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engine",
		Short: "Manage the local engine",
		Long: `Manage the local engine.

Local contexts run filament-standalone, which this CLI downloads for its own
version on first use. FILAMENT_ENGINE points at a different binary.`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Download the local engine for this version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := engine.Locate(cmd.Context(), version.Version, a.statusWriter())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(a.stdout, path)
			return err
		},
	})
	return cmd
}
