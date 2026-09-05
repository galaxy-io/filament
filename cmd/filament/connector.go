package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

const connectionGuidance = `Connector fields are set with --%[1]s-<field>. Secret fields accept plaintext
values, quoted $NAME or ${NAME} references, or --%[1]s-<field>-env VARIABLE.
Editing removes a field with --unset %[1]s-<field>.`

func (a *cliApp) connectionCommand(kind string) *cobra.Command {
	cmd := &cobra.Command{
		Use:               kind,
		Short:             fmt.Sprintf("Manage saved %ss", kind),
		Long:              fmt.Sprintf("Manage saved %ss.\n\n"+connectionGuidance, kind),
		PersistentPreRunE: a.prepareTarget,
	}
	help := func(operation string) func(context.Context, []string) error {
		return func(ctx context.Context, args []string) error {
			return a.printConnectionOperationHelp(ctx, kind, operation, args)
		}
	}
	change := func(operation string) func(context.Context, []string) error {
		return func(ctx context.Context, args []string) error {
			return a.changeConnection(ctx, operation, kind, args)
		}
	}
	cmd.AddCommand(
		a.dynamicCommand(
			fmt.Sprintf("create <name> --%s-connector NAME [flags]", kind),
			"Save a "+kind, help("create"), change("create"),
		),
		a.dynamicCommand(
			fmt.Sprintf("edit <name> [flags] [--unset %s-FIELD]", kind),
			"Change a saved "+kind, help("edit"), change("edit"),
		),
		a.connectionListCommand(kind),
	)
	if kind == "source" {
		cmd.AddCommand(a.dynamicCommand(
			"discover [name] [--source-connector NAME] [flags]",
			"List the resources a source exposes", help("discover"), a.discoverSource,
		))
	}
	remove := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved " + kind,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return a.deleteConnection(cmd.Context(), kind, args[0], force)
		},
	}
	remove.Flags().Bool("force", false, "Skip the confirmation prompt")
	cmd.AddCommand(remove)
	return cmd
}

func (a *cliApp) changeConnection(ctx context.Context, operation, kind string, args []string) error {
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return err
	}
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

func (a *cliApp) deleteConnection(ctx context.Context, kind, name string, force bool) error {
	document, err := a.service.Configuration(ctx)
	if err != nil {
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

// connectionUsage maps each saved connection to the pipelines referencing it.
func (a *cliApp) connectionUsage(ctx context.Context, kind string) (map[string][]string, error) {
	pipelines, err := a.service.Pipelines(ctx)
	if err != nil {
		return nil, err
	}
	usage := map[string][]string{}
	for _, pipeline := range pipelines.Items {
		ref := pipeline.Source
		if kind == "sink" {
			ref = pipeline.Sink
		}
		usage[ref] = append(usage[ref], pipeline.Name)
	}
	return usage, nil
}

func (a *cliApp) configName() string {
	return filepath.Base(a.service.ConfigurationLocation())
}

func (a *cliApp) connectionListCommand(kind string) *cobra.Command {
	var page listPageFlags
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   fmt.Sprintf("List saved %ss", kind),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request, err := page.request()
			if err != nil {
				return err
			}
			result, err := a.service.ConnectionPage(cmd.Context(), kind, request)
			if err != nil {
				return err
			}
			usedBy, err := a.connectionUsage(cmd.Context(), kind)
			if err != nil {
				return err
			}
			return a.text().Connections(result, a.configName(), usedBy, "filament "+kind+" list")
		},
	}
	page.add(cmd)
	return cmd
}
