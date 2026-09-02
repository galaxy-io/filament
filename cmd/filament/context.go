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
	if profile := target.Target.AuthProfile; profile != "" && !a.profileInUse(registry, profile) {
		_ = (cliauth.Store{Path: a.credentialsPath()}).Delete(profile)
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("Deleted context %s", name))
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
	return textrenderer.Contexts(a.stdout, result, filepath.Base(a.contextPath))
}

func (a *cliApp) contextRegistry() *contexts.Registry {
	return contexts.NewRegistry(contexts.Store{Path: a.contextPath}, a.configPath)
}

func (a *cliApp) contextAddCommand() *cobra.Command {
	var endpoint, authProfile string
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a remote or local context",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// The global --config flag names the document a local context serves.
			if (endpoint == "") == !a.configOverride {
				return fmt.Errorf("exactly one of --server (remote) or --config (local) is required")
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
			if err := a.contextRegistry().Set(args[0], target); err != nil {
				return err
			}
			return printSuccess(a.statusWriter(), fmt.Sprintf("Added context %s", args[0]))
		},
	}
	add.Flags().StringVar(&endpoint, "server", "", "Filament server `URL`")
	add.Flags().StringVar(&authProfile, "auth-profile", "", "Stored authentication profile `NAME`")
	return add
}
