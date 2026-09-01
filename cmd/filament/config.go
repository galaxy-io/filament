package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func (a *cliApp) configCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "config",
		Short:             "Work with the configuration file",
		PersistentPreRunE: a.prepareTarget,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "path",
			Short: "Print the configuration file path",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				location := a.service.ConfigurationLocation()
				if location == "" {
					return cliapp.ErrRawConfigurationUnsupported
				}
				_, err := fmt.Fprintln(a.stdout, location)
				return err
			},
		},
		&cobra.Command{
			Use:   "validate",
			Short: "Check the configuration file",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if err := a.service.ValidateConfiguration(cmd.Context()); err != nil {
					return err
				}
				_, err := fmt.Fprintf(a.statusWriter(), "%s is structurally valid.\n", a.service.ConfigurationLocation())
				return err
			},
		},
		&cobra.Command{
			Use:   "edit",
			Short: "Open the configuration file in $EDITOR",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return a.editConfig(cmd.Context())
			},
		},
	)
	return cmd
}

func (a *cliApp) editConfig(ctx context.Context) error {
	data, err := a.service.ReadConfiguration(ctx)
	if err != nil {
		return err
	}
	recovery, err := os.CreateTemp("", "filament-recovery-*.yaml")
	if err != nil {
		return err
	}
	recoveryPath := recovery.Name()
	if _, err := recovery.Write(data); err != nil {
		_ = recovery.Close()
		return err
	}
	if err := recovery.Close(); err != nil {
		return err
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	// #nosec G204,G702 -- VISUAL and EDITOR intentionally select the user's editor;
	// CommandContext invokes it directly without a shell.
	cmd := exec.CommandContext(ctx, parts[0], append(parts[1:], recoveryPath)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.stdin, a.stdout, a.statusWriter()
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed; recovery file kept at %s: %w", recoveryPath, err)
	}
	data, err = os.ReadFile(recoveryPath)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	doc := climodel.NewDocument()
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("edited config is invalid; recovery file kept at %s: %w", recoveryPath, err)
	}
	doc.Normalize()
	if err := cliapp.ValidateDocument(doc, a.catalog); err != nil {
		return fmt.Errorf("edited config is invalid; recovery file kept at %s: %w", recoveryPath, err)
	}
	if err := a.service.WriteConfiguration(ctx, data); err != nil {
		return fmt.Errorf("recovery file kept at %s: %w", recoveryPath, err)
	}
	_ = os.Remove(recoveryPath)
	_, err = fmt.Fprintf(a.statusWriter(), "Updated %s.\n", a.service.ConfigurationLocation())
	return err
}
