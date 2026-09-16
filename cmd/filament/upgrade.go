package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	cliupgrade "github.com/galaxy-io/filament/cmd/internal/cli/upgrade"
	"github.com/galaxy-io/filament/cmd/internal/version"
)

const homebrewFormula = "galaxy-io/tap/filament"

func (a *cliApp) upgradeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the Filament CLI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate filament executable: %w", err)
			}
			return a.runUpgrade(cmd.Context(), executable)
		},
	}
}

func (a *cliApp) runUpgrade(ctx context.Context, executable string) error {
	loader := a.renderer()
	interactive := loader.Interactive()
	var result cliupgrade.Result
	err := loader.RunLoading("Checking for updates to latest version...", func(setLabel func(string)) error {
		var err error
		result, err = cliupgrade.Run(ctx, cliupgrade.Options{
			Client:         &http.Client{Timeout: 2 * time.Minute},
			CurrentVersion: version.Version,
			Executable:     executable,
			Status: func(message string) error {
				if interactive {
					setLabel(message)
					return nil
				}
				_, err := fmt.Fprintln(a.stdout, message)
				return err
			},
		})
		return err
	})
	if errors.Is(err, cliupgrade.ErrHomebrewManaged) {
		return a.runHomebrewUpgrade(ctx)
	}
	if err != nil {
		return err
	}
	if !result.Updated {
		_, err = fmt.Fprintf(a.stdout, "Filament is up to date (%s)\n", result.Latest)
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "Upgraded filament %s → %s\n", result.Current, result.Latest)
	return err
}

func (a *cliApp) runHomebrewUpgrade(ctx context.Context) error {
	if _, err := fmt.Fprintf(a.stdout, "Updating Filament via `brew upgrade %s`...\n", homebrewFormula); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "brew", "upgrade", homebrewFormula)
	command.Stdin = a.stdin
	command.Stdout = a.stdout
	command.Stderr = a.statusWriter()
	if err := command.Run(); err != nil {
		return fmt.Errorf("brew upgrade %s: %w", homebrewFormula, err)
	}
	return nil
}
