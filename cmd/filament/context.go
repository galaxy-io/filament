package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	cliauth "github.com/galaxy-io/filament/cmd/internal/cli/auth"
	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) contextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "context",
		Aliases: []string{"ctx"},
		Short:   "Manage contexts",
		Long: `Manage contexts.

Contexts are stored separately from pipeline configuration and credentials.
Use --context NAME to select a context for one invocation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 {
				selected, err := a.contextRegistry().Use(args[0])
				if err != nil {
					return err
				}
				return printSuccess(a.statusWriter(), fmt.Sprintf("Switched to context %s", selected.Name))
			}
			current, err := a.contextRegistry().Resolve(a.contextName)
			if err != nil {
				return err
			}
			return textrenderer.CurrentContext(a.stdout, current.Name)
		},
	}
	add := a.contextAddCommand()
	cmd.AddCommand(
		add,
		&cobra.Command{
			Use:     "list",
			Aliases: []string{"ls"},
			Short:   "List contexts",
			Args:    cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return a.listContexts(a.contextRegistry())
			},
		},
		&cobra.Command{
			Use:   "current",
			Short: "Print the current context",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				current, err := a.contextRegistry().Resolve(a.contextName)
				if err != nil {
					return err
				}
				return textrenderer.CurrentContext(a.stdout, current.Name)
			},
		},
		&cobra.Command{
			Use:   "use <name>",
			Short: "Switch the current context",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				selected, err := a.contextRegistry().Use(args[0])
				if err != nil {
					return err
				}
				return printSuccess(a.statusWriter(), fmt.Sprintf("Switched to context %s", selected.Name))
			},
		},
		&cobra.Command{
			Use:   "rename <name> <new-name>",
			Short: "Rename a context",
			Args:  cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := a.contextRegistry().Rename(args[0], args[1]); err != nil {
					return err
				}
				return printSuccess(a.statusWriter(), fmt.Sprintf("Renamed context %s to %s", args[0], args[1]))
			},
		},
		&cobra.Command{
			Use:   "delete <name>",
			Short: "Delete a context",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				return a.deleteContext(args[0])
			},
		},
	)
	return cmd
}

// deleteContext removes a context and, when no other context shares them,
// its stored credentials.
func (a *cliApp) deleteContext(name string) error {
	registry := a.contextRegistry()
	target, err := registry.Resolve(name)
	if err != nil {
		return err
	}
	if err := registry.Delete(name); err != nil {
		return err
	}
	if err := a.removeUnusedProfile(registry, target.Target.AuthProfile); err != nil {
		return fmt.Errorf("deleted context %s, but %w", name, err)
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("Deleted context %s", name))
}

// removeUnusedProfile deletes stored credentials once no context references
// them. Call it after the referencing context is gone from the registry.
func (a *cliApp) removeUnusedProfile(registry *contexts.Registry, profile string) error {
	if profile == "" || a.profileInUse(registry, profile) {
		return nil
	}
	if err := (cliauth.Store{Path: a.credentialsPath()}).Delete(profile); err != nil {
		return fmt.Errorf("removing auth profile %s: %w", profile, err)
	}
	return nil
}

func (a *cliApp) profileInUse(registry *contexts.Registry, profile string) bool {
	remaining, err := registry.List()
	if err != nil {
		return true
	}
	for _, item := range remaining {
		if item.Target.AuthProfile == profile {
			return true
		}
	}
	return false
}

func (a *cliApp) listContexts(registry *contexts.Registry) error {
	items, err := registry.List()
	if err != nil {
		return err
	}
	selected := ""
	if a.contextName != "" {
		effective, err := registry.Resolve(a.contextName)
		if err != nil {
			return err
		}
		selected = effective.Name
	}
	result := climodel.ContextList{Items: make([]climodel.ContextSummary, 0, len(items))}
	for _, item := range items {
		current := item.Current
		if selected != "" {
			current = item.Name == selected
		}
		location := item.Target.Endpoint
		if item.Target.Kind == contexts.KindLocal {
			location = item.Target.ConfigPath
		}
		result.Items = append(result.Items, climodel.ContextSummary{
			Name: item.Name, Current: current, Kind: string(item.Target.Kind),
			Location: location,
		})
	}
	return a.text().Contexts(result, filepath.Base(a.contextPath))
}

func (a *cliApp) contextRegistry() *contexts.Registry {
	return contexts.NewRegistry(contexts.Store{Path: a.contextPath}, a.configPath)
}

func (a *cliApp) contextAddCommand() *cobra.Command {
	var endpoint, authProfile string
	var force bool
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a remote or local context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			// The global --config flag names the document a local context serves.
			remote, local := endpoint != "", a.configOverride
			if remote == local {
				return fmt.Errorf("exactly one of --server (remote) or --config (local) is required")
			}
			if authProfile != "" {
				if _, err := (cliauth.Store{Path: a.credentialsPath()}).Get(authProfile); err != nil {
					return err
				}
			}
			target := contexts.Target{
				Kind: contexts.KindRemote, Endpoint: endpoint, AuthProfile: authProfile,
			}
			if a.configOverride {
				absolute, err := filepath.Abs(a.configPath)
				if err != nil {
					return err
				}
				target = contexts.Target{Kind: contexts.KindLocal, ConfigPath: absolute}
			}
			registry := a.contextRegistry()
			existing, err := registry.Resolve(name)
			replacing := err == nil
			if replacing && !force {
				confirmed, err := a.confirm(fmt.Sprintf("Context %q already exists (%s); replace it?", name, existing.Target.Kind))
				if err != nil || !confirmed {
					return err
				}
			}
			if err := registry.Set(name, target); err != nil {
				return err
			}
			if replacing {
				if err := a.removeUnusedProfile(registry, existing.Target.AuthProfile); err != nil {
					return fmt.Errorf("replaced context %s, but %w", name, err)
				}
				return printSuccess(a.statusWriter(), fmt.Sprintf("Replaced context %s", name))
			}
			return printSuccess(a.statusWriter(), fmt.Sprintf("Added context %s", name))
		},
	}
	add.Flags().StringVar(&endpoint, "server", "", "Filament server `URL`")
	add.Flags().StringVar(&authProfile, "auth-profile", "", "Stored authentication profile `NAME`")
	add.Flags().BoolVar(&force, "force", false, "Replace an existing context without confirmation")
	return add
}
