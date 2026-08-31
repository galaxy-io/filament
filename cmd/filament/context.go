package main

import (
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/cli/contexts"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

const contextHelp = `Context commands

Usage:
  filament context list
  filament context current
  filament context use <name>

Contexts are stored separately from pipeline configuration and credentials.
Use --context NAME to select a context for one invocation.
`

func (a *cliApp) runContextCommand(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" || helpRequested(args[1:]) {
		_, err := fmt.Fprint(a.stdout, contextHelp)
		return err
	}
	registry := a.contextRegistry()
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: filament context list")
		}
		return a.listContexts(registry)
	case "current":
		if len(args) != 1 {
			return fmt.Errorf("usage: filament context current")
		}
		current, err := registry.Resolve(a.contextName)
		if err != nil {
			return err
		}
		return textrenderer.CurrentContext(a.stdout, current.Name)
	case "use":
		if len(args) != 2 {
			return fmt.Errorf("usage: filament context use <name>")
		}
		selected, err := registry.Use(args[1])
		if err != nil {
			return err
		}
		return printSuccess(a.statusWriter(), fmt.Sprintf("Switched to context %s", selected.Name))
	default:
		return fmt.Errorf("unknown context operation %q", args[0])
	}
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
	return textrenderer.Contexts(a.stdout, result)
}

func (a *cliApp) contextRegistry() *contexts.Registry {
	return contexts.NewRegistry(contexts.Store{Path: a.contextPath}, a.configPath)
}
