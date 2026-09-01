package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) contextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage contexts",
		Long: `Manage contexts.

Contexts are stored separately from pipeline configuration and credentials.
Use --context NAME to select a context for one invocation.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List contexts",
			Args:  cobra.NoArgs,
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
	)
	return cmd
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
			Location: location, Tenant: item.Target.Tenant,
		})
	}
	return textrenderer.Contexts(a.stdout, result, filepath.Base(a.contextPath))
}

func (a *cliApp) contextRegistry() *contexts.Registry {
	return contexts.NewRegistry(contexts.Store{Path: a.contextPath}, a.configPath)
}
