package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

const resourcesGuidance = "An omitted --resources selection means all resources discovered by the source."

const pipelineGuidance = resourcesGuidance + `
Connector pipeline fields use --source-<field> and --sink-<field>. Repeat
--unset to remove optional connector fields from an existing pipeline.`

type helpOutput struct {
	w   io.Writer
	err error
}

func (o *helpOutput) printf(format string, values ...any) {
	if o.err == nil {
		_, o.err = fmt.Fprintf(o.w, format, values...)
	}
}

// usage prints each form of cmd's use line, then its subcommands.
func (o *helpOutput) usage(paint style.Painter, cmd *cobra.Command) {
	o.printf("%s\n", paint.Bold("Usage:"))
	path := cmd.CommandPath()
	for _, form := range strings.Split(strings.TrimPrefix(cmd.Use, cmd.Name()+" "), " | ") {
		o.printf("  %s %s\n", paint.Accent(path), paint.Muted(form))
	}
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			o.printf("  %s %s\n", paint.Accent(sub.CommandPath()), paint.Muted(strings.TrimPrefix(sub.UseLine(), sub.CommandPath()+" ")))
		}
	}
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "help" || arg == "--help" || arg == "-h" || strings.HasPrefix(arg, "--help=") {
			return true
		}
	}
	return false
}

// helpArgs parses the arguments a help request came with. Errors are ignored
// so partial input still selects which connector flags to show.
func (a *cliApp) helpArgs(args []string) commandArgs {
	kept := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "help" && arg != "--help" && arg != "-h" && !strings.HasPrefix(arg, "--help=") {
			kept = append(kept, arg)
		}
	}
	parsed, _ := a.parseCommandArgs(kept)
	return parsed
}

func (a *cliApp) printConnectionOperationHelp(ctx context.Context, cmd *cobra.Command, kind, operation string, args []string) error {
	out := &helpOutput{w: a.stdout}
	paint := style.New(a.stdout)
	parsed := a.helpArgs(args)
	connector := lastFlag(parsed.flags, kind+"-connector")
	name := firstPositional(parsed)
	if connector == "" && name != "" && a.service != nil {
		if connection, err := a.service.Connection(ctx, kind, name); err == nil {
			connector = connection.Type
		}
	}
	out.usage(paint, cmd)
	if connector == "" {
		out.printf("\n%s %s\n", paint.Bold("Available "+kind+" connectors:"), strings.Join(a.catalog.Names(kind), ", "))
		return out.err
	}
	schema, err := a.catalog.ConnectionSchema(kind, connector)
	if err != nil {
		out.printf("\nUnknown %s connector %q.\n", kind, connector)
		return out.err
	}
	scope := filament.ScopeConnection
	if operation == "discover" && name != "" {
		scope = filament.ScopePipeline
	}
	out.printf("\n%s\n", paint.Bold(connector+" connector flags:"))
	printSchemaFlags(out, paint, kind+"-", schema, scope, operation == "discover" && name == "")
	return out.err
}

func (a *cliApp) printRunHelp(ctx context.Context, cmd *cobra.Command, args []string) error {
	out := &helpOutput{w: a.stdout}
	paint := style.New(a.stdout)
	out.usage(paint, cmd)
	out.printf("\n%s\n", paint.Muted(resourcesGuidance+"\nUse --source-<field> and --sink-<field> for connector configuration."))
	parsed := a.helpArgs(args)
	printed := map[string]bool{}
	for _, kind := range []string{"source", "sink"} {
		connector := lastFlag(parsed.flags, kind+"-connector")
		if connector == "" {
			continue
		}
		schema, err := a.catalog.ConnectionSchema(kind, connector)
		if err != nil {
			continue
		}
		out.printf("\n%s\n", paint.Bold(connector+" "+kind+" flags:"))
		printSchemaFlags(out, paint, kind+"-", schema, filament.ScopeConnection, true)
		printed[kind] = true
	}
	if name := firstPositional(parsed); name != "" && a.service != nil {
		if pipeline, err := a.service.Pipeline(ctx, name); err == nil {
			a.printPipelineFlags(ctx, out, paint, pipeline.Source.Ref, pipeline.Sink.Ref, printed)
		}
	}
	return out.err
}

func (a *cliApp) printPipelineOperationHelp(ctx context.Context, cmd *cobra.Command, args []string) error {
	out := &helpOutput{w: a.stdout}
	paint := style.New(a.stdout)
	out.usage(paint, cmd)
	out.printf("\n%s\n", paint.Muted(pipelineGuidance))
	parsed := a.helpArgs(args)
	sourceRef := lastFlag(parsed.flags, "source")
	sinkRef := lastFlag(parsed.flags, "sink")
	if name := firstPositional(parsed); name != "" && a.service != nil {
		if pipeline, err := a.service.Pipeline(ctx, name); err == nil {
			sourceRef = cmp.Or(sourceRef, pipeline.Source.Ref)
			sinkRef = cmp.Or(sinkRef, pipeline.Sink.Ref)
		}
	}
	a.printPipelineFlags(ctx, out, paint, sourceRef, sinkRef, nil)
	return out.err
}

// printPipelineFlags prints the pipeline-scoped flags of the saved source and
// sink a pipeline routes between, skipping kinds already printed.
func (a *cliApp) printPipelineFlags(ctx context.Context, out *helpOutput, paint style.Painter, sourceRef, sinkRef string, skip map[string]bool) {
	if a.service == nil {
		return
	}
	for _, side := range []struct{ kind, ref string }{{"source", sourceRef}, {"sink", sinkRef}} {
		if side.ref == "" || skip[side.kind] {
			continue
		}
		connection, err := a.service.Connection(ctx, side.kind, side.ref)
		if err != nil {
			continue
		}
		schema, err := a.catalog.ConnectionSchema(side.kind, connection.Type)
		if err != nil {
			continue
		}
		out.printf("\n%s\n", paint.Bold(connection.Type+" "+side.kind+" flags:"))
		printSchemaFlags(out, paint, side.kind+"-", schema, filament.ScopePipeline, false)
	}
}

func printSchemaFlags(out *helpOutput, paint style.Painter, prefix string, schema filament.ConfigSchema, scope filament.FieldScope, allScopes bool) {
	rows := []flagRow{}
	add := func(name, placeholder, usage string) {
		rows = append(rows, flagRow{plain: name + " " + placeholder, painted: paint.Accent(name) + " " + paint.Muted(placeholder), usage: usage})
	}
	for _, field := range schema.Fields {
		actualScope := field.Scope
		if actualScope == filament.ScopeUnspecified {
			actualScope = filament.ScopeConnection
		}
		if !allScopes && actualScope != scope {
			continue
		}
		name := "--" + prefix + strings.ReplaceAll(field.Name, "_", "-")
		usage := field.Help
		if field.Required && field.Default == nil {
			usage += " " + paint.Muted("(required)")
		}
		add(name, fieldTypeName(field.Type), usage)
		if cliapp.IsSecretField(field) {
			add(name+"-env", "VARIABLE", "environment-variable reference (NAME, $NAME, or ${NAME})")
		}
	}
	out.printf("%s", alignFlagRows(rows))
}

func fieldTypeName(t filament.FieldType) string {
	switch t {
	case filament.FieldInt:
		return "INT"
	case filament.FieldBool:
		return "[BOOL]"
	case filament.FieldObject:
		return "JSON"
	case filament.FieldList:
		return "LIST"
	default:
		return "VALUE"
	}
}
