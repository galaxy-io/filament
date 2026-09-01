package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func (a *cliApp) runConfigCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" || helpRequested(args[1:]) {
		_, err := fmt.Fprint(a.stdout, configHelp)
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: filament config <path|validate|edit>")
	}
	location := a.service.ConfigurationLocation()
	switch args[0] {
	case "path":
		if location == "" {
			return cliapp.ErrRawConfigurationUnsupported
		}
		_, err := fmt.Fprintln(a.stdout, location)
		return err
	case "validate":
		if err := a.service.ValidateConfiguration(ctx); err != nil {
			return err
		}
		_, err := fmt.Fprintf(a.statusWriter(), "%s is structurally valid.\n", location)
		return err
	case "edit":
		return a.editConfig(ctx)
	default:
		return fmt.Errorf("unknown config operation %q", args[0])
	}
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
