package main

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) runConnectionCommand(ctx context.Context, kind string, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return a.printConnectionHelp(kind)
	}
	if helpRequested(args[1:]) && rawFlagValue(args[1:], kind+"-connector") != "" {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], climodel.NewDocument())
	}
	if args[0] == "list" && helpRequested(args[1:]) {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], climodel.NewDocument())
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return fmt.Errorf("usage: filament %s list", kind)
		}
		result, err := a.service.Connections(ctx, kind)
		if err != nil {
			return err
		}
		return textrenderer.Connections(a.stdout, result)
	}
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if helpRequested(args[1:]) {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], document)
	}
	switch args[0] {
	case "discover":
		if kind != "source" {
			return fmt.Errorf("discover is only available for sources")
		}
		return a.discoverSource(ctx, args[1:], document)
	case "create", "edit":
		return a.changeConnection(ctx, args[0], kind, args[1:], document)
	case "delete":
		return a.deleteConnection(ctx, kind, args[1:], document)
	default:
		return fmt.Errorf("unknown %s operation %q", kind, args[0])
	}
}

func (a *cliApp) changeConnection(ctx context.Context, operation, kind string, args []string, document climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	if name == "" {
		return fmt.Errorf("%s name is required; example: filament %s %s production --%s-connector postgres --%s-dsn-env POSTGRES_DSN", kind, kind, operation, kind, kind)
	}
	var existing *climodel.Connection
	if current, ok := connectionMap(kind, document)[name]; ok {
		existing = &current
	}
	if operation == "edit" && existing == nil {
		return fmt.Errorf("%s %q does not exist", kind, name)
	}
	request, err := a.connectionRequestFromFlags(operation, kind, name, existing, parsed.flags)
	if err != nil {
		return err
	}
	if _, err := a.service.SaveConnection(ctx, request); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("%s %s %s", pastTense(operation), name, kind))
}

func (a *cliApp) deleteConnection(ctx context.Context, kind string, args []string, document climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	if name == "" {
		return fmt.Errorf("usage: filament %s delete <name> [--force]", kind)
	}
	force, err := parseForceFlag(parsed.flags)
	if err != nil {
		return err
	}
	if err := rejectUnknownFlags(parsed.flags, map[string]bool{"force": true}); err != nil {
		return err
	}
	if _, ok := connectionMap(kind, document)[name]; !ok {
		return fmt.Errorf("%s %q does not exist", kind, name)
	}
	if !force {
		confirmed, err := a.confirmDelete(kind, name)
		if err != nil || !confirmed {
			return err
		}
	}
	if err := a.service.DeleteSavedConnection(ctx, kind, name); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.statusWriter(), "Deleted %s %q.\n", kind, name)
	return err
}

func (a *cliApp) connectionRequestFromFlags(operation, kind, name string, existing *climodel.Connection, flags map[string][]string) (cliapp.SaveConnectionRequest, error) {
	prefix := kind + "-"
	connectorFlag := prefix + "connector"
	connector := lastFlag(flags, connectorFlag)
	if connector == "" && existing != nil {
		connector = existing.Type
	}
	request := cliapp.SaveConnectionRequest{
		Create: operation == "create", Kind: kind, Name: name, Connector: connector,
	}
	if connector == "" {
		return request, fmt.Errorf("%s %q: --%s is required", kind, name, connectorFlag)
	}
	schema, err := a.catalog.ConnectionSchema(kind, connector)
	if err != nil {
		return request, err
	}
	allowed := map[string]bool{connectorFlag: true}
	patch, unsetNames, err := configPatchFromFlags(schema, filament.ScopeConnection, prefix, flags, allowed, true)
	if err != nil {
		return request, err
	}
	unsetTargets := map[string]func(){}
	for flagName, configName := range unsetNames {
		configName := configName
		unsetTargets[flagName] = func() { patch.Unset = append(patch.Unset, configName) }
	}
	if err := applyFieldUnsets(flags, unsetTargets, allowed); err != nil {
		return request, err
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return request, err
	}
	request.Config = patch
	return request, nil
}

func connectionMap(kind string, document climodel.Document) map[string]climodel.Connection {
	if kind == "sink" {
		return document.Sinks
	}
	return document.Sources
}
