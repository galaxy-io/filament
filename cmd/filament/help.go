package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

type helpOutput struct {
	w   io.Writer
	err error
}

func (o *helpOutput) print(value string) {
	if o.err == nil {
		_, o.err = fmt.Fprint(o.w, value)
	}
}

func (o *helpOutput) printf(format string, values ...any) {
	if o.err == nil {
		_, o.err = fmt.Fprintf(o.w, format, values...)
	}
}

func (o *helpOutput) usage(paint style.Painter, command, rest string) {
	o.printf("%s\n  %s %s\n", paint.Bold("Usage:"), paint.Accent(command), paint.Muted(rest))
}

func helpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "help" || arg == "--help" || arg == "-h" || strings.HasPrefix(arg, "--help=") {
			return true
		}
	}
	return false
}

func removeHelp(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "help" && arg != "--help" && arg != "-h" && !strings.HasPrefix(arg, "--help=") {
			result = append(result, arg)
		}
	}
	return result
}

func rawFlagValue(args []string, name string) string {
	flag := "--" + name
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], flag+"=") {
			return strings.TrimPrefix(args[i], flag+"=")
		}
	}
	return ""
}

func (a *cliApp) printConnectionOperationHelp(ctx context.Context, kind, operation string, args []string) error {
	doc := climodel.NewDocument()
	if a.service != nil && rawFlagValue(args, kind+"-connector") == "" {
		loaded, err := a.service.Configuration(ctx)
		if err != nil {
			return err
		}
		doc = loaded
	}
	out := &helpOutput{w: a.stdout}
	prefix := kind + "-"
	connectorName := rawFlagValue(args, prefix+"connector")
	parsed, _ := a.parseCommandArgs(removeHelp(args))
	name := firstPositional(parsed)
	if connectorName == "" && name != "" {
		if conn, ok := connectionMap(kind, doc)[name]; ok {
			connectorName = conn.Type
		}
	}

	paint := style.New(a.stdout)
	switch operation {
	case "create":
		out.usage(paint, fmt.Sprintf("filament %s create", kind), fmt.Sprintf("<name> --%sconnector NAME [flags]", prefix))
	case "edit":
		out.usage(paint, fmt.Sprintf("filament %s edit", kind), fmt.Sprintf("<name> [flags] [--unset %sFIELD]", prefix))
	case "discover":
		out.usage(paint, "filament source discover", "[name] [--source-connector NAME] [flags]")
	}

	if connectorName == "" {
		out.printf("\n%s %s\n", paint.Bold("Available "+kind+" connectors:"), strings.Join(a.connectorNames(kind), ", "))
		return out.err
	}
	schema, ok := a.connectorSchema(kind, connectorName)
	if !ok {
		out.printf("\nUnknown %s connector %q.\n", kind, connectorName)
		return out.err
	}
	scope := filament.ScopeConnection
	if operation == "discover" && name != "" {
		scope = filament.ScopePipeline
	}
	out.printf("\n%s\n", paint.Bold(connectorName+" connector flags:"))
	printSchemaFlags(out, paint, prefix, schema, scope, operation == "discover" && name == "")
	return out.err
}

func (a *cliApp) printRunHelp(ctx context.Context, args []string) error {
	out := &helpOutput{w: a.stdout}
	paint := style.New(a.stdout)
	out.print(paint.Bold("Usage:") + runHelp)
	printed := map[string]bool{}
	for _, kind := range []string{"source", "sink"} {
		name := rawFlagValue(args, kind+"-connector")
		if name == "" {
			continue
		}
		schema, ok := a.connectorSchema(kind, name)
		if !ok {
			continue
		}
		out.printf("\n%s\n", paint.Bold(name+" "+kind+" flags:"))
		printSchemaFlags(out, paint, kind+"-", schema, filament.ScopeConnection, true)
		printed[kind] = true
	}
	parsed, _ := a.parseCommandArgs(removeHelp(args))
	if name := firstPositional(parsed); name != "" && a.service != nil {
		if doc, err := a.service.Configuration(ctx); err == nil {
			if p, ok := doc.Pipelines[name]; ok {
				for _, item := range []struct{ kind, ref string }{{"source", p.Source.Ref}, {"sink", p.Sink.Ref}} {
					kind, ref := item.kind, item.ref
					if printed[kind] {
						continue
					}
					conn, ok := connectionMap(kind, doc)[ref]
					if !ok {
						continue
					}
					schema, ok := a.connectorSchema(kind, conn.Type)
					if ok {
						out.printf("\n%s\n", paint.Bold(conn.Type+" "+kind+" flags:"))
						printSchemaFlags(out, paint, kind+"-", schema, filament.ScopePipeline, false)
					}
				}
			}
		}
	}
	return out.err
}

func (a *cliApp) printPipelineOperationHelp(ctx context.Context, args []string) error {
	doc := climodel.NewDocument()
	if a.service != nil {
		loaded, err := a.service.Configuration(ctx)
		if err != nil {
			return err
		}
		doc = loaded
	}
	out := &helpOutput{w: a.stdout}
	paint := style.New(a.stdout)
	out.print(paint.Bold("Usage:") + pipelineHelp)
	parsed, _ := a.parseCommandArgs(removeHelp(args))
	sourceRef := lastFlag(parsed.flags, "source")
	sinkRef := lastFlag(parsed.flags, "sink")
	if name := firstPositional(parsed); name != "" {
		if p, ok := doc.Pipelines[name]; ok {
			if sourceRef == "" {
				sourceRef = p.Source.Ref
			}
			if sinkRef == "" {
				sinkRef = p.Sink.Ref
			}
		}
	}
	for _, item := range []struct{ kind, ref string }{{"source", sourceRef}, {"sink", sinkRef}} {
		kind, ref := item.kind, item.ref
		if ref == "" {
			continue
		}
		conn, ok := connectionMap(kind, doc)[ref]
		if !ok {
			continue
		}
		schema, ok := a.connectorSchema(kind, conn.Type)
		if !ok {
			continue
		}
		out.printf("\n%s\n", paint.Bold(conn.Type+" "+kind+" flags:"))
		printSchemaFlags(out, paint, kind+"-", schema, filament.ScopePipeline, false)
	}
	return out.err
}

func (a *cliApp) connectorNames(kind string) []string {
	if kind == "sink" {
		return a.catalog.SinkNames()
	}
	return a.catalog.SourceNames()
}

func (a *cliApp) connectorSchema(kind, name string) (filament.ConfigSchema, bool) {
	if kind == "sink" {
		spec, ok := a.catalog.Sinks[name]
		return spec.Config, ok
	}
	spec, ok := a.catalog.Sources[name]
	return spec.Config, ok
}

func printSchemaFlags(out *helpOutput, paint style.Painter, prefix string, schema filament.ConfigSchema, scope filament.FieldScope, allScopes bool) {
	type row struct {
		plain, painted, help string
	}
	rows := []row{}
	add := func(name, placeholder, help string) {
		rows = append(rows, row{plain: name + " " + placeholder, painted: paint.Accent(name) + " " + paint.Muted(placeholder), help: help})
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
		help := field.Help
		if field.Required && field.Default == nil {
			help += " " + paint.Muted("(required)")
		}
		add(name, fieldTypeName(field.Type), help)
		if cliapp.IsSecretField(field) {
			add(name+"-env", "VARIABLE", "environment-variable reference (NAME, $NAME, or ${NAME})")
		}
	}
	width := 0
	for _, item := range rows {
		width = max(width, len(item.plain))
	}
	for _, item := range rows {
		out.printf("  %s%s   %s\n", item.painted, strings.Repeat(" ", width-len(item.plain)), item.help)
	}
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

const pipelineHelp = `
  filament pipeline create <name> --source NAME --sink NAME [--resources LIST] [flags]
  filament pipeline edit <name> [flags] [--unset source-FIELD|sink-FIELD]
  filament pipeline list
  filament pipeline delete <name> [--force]

An omitted --resources selection means all resources discovered by the source.
Connector pipeline fields use --source-<field> and --sink-<field>.
Repeat --unset to remove optional connector fields from an existing pipeline.
`

const runHelp = `
  filament run <pipeline> [--resources LIST] [--sync-mode full] [--write-mode MODE] [--unset KIND-FIELD]
  filament run --source-connector NAME --sink-connector NAME [flags]

An omitted --resources selection means all resources discovered by the source.
Use --source-<field> and --sink-<field> for connector configuration.
`
