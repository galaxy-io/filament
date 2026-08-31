package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func (a *cliApp) runConfigCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" || helpRequested(args[1:]) {
		_, err := fmt.Fprint(a.stdout, configHelp)
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: filament config <path|validate|edit>")
	}
	store := configStore{path: a.configPath}
	switch args[0] {
	case "path":
		_, err := fmt.Fprintln(a.stdout, a.configPath)
		return err
	case "validate":
		doc, _, err := store.load()
		if err != nil {
			return err
		}
		if err := validateDocument(doc, a.catalog); err != nil {
			return err
		}
		_, err = fmt.Fprintf(a.statusWriter(), "%s is structurally valid.\n", a.configPath)
		return err
	case "edit":
		return a.editConfig(ctx, store)
	default:
		return fmt.Errorf("unknown config operation %q", args[0])
	}
}

func (a *cliApp) editConfig(ctx context.Context, store configStore) error {
	doc, root, err := store.load()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return err
	}
	recovery, err := os.CreateTemp(filepath.Dir(store.path), "filament-recovery-*.yaml")
	if err != nil {
		return err
	}
	recoveryPath := recovery.Name()
	encoder := yaml.NewEncoder(recovery)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		_ = recovery.Close()
		return err
	}
	_ = encoder.Close()
	_ = recovery.Close()
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
	data, err := os.ReadFile(recoveryPath)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("edited config is invalid; recovery file kept at %s: %w", recoveryPath, err)
	}
	doc.Normalize()
	if err := validateDocument(doc, a.catalog); err != nil {
		return fmt.Errorf("edited config is invalid; recovery file kept at %s: %w", recoveryPath, err)
	}
	var editedRoot yaml.Node
	if err := yaml.Unmarshal(data, &editedRoot); err != nil {
		return err
	}
	if err := store.write(&editedRoot); err != nil {
		return fmt.Errorf("recovery file kept at %s: %w", recoveryPath, err)
	}
	_ = os.Remove(recoveryPath)
	_, err = fmt.Fprintf(a.statusWriter(), "Updated %s.\n", store.path)
	return err
}
