package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var rootHelp = `Filament CLI

Usage:
  filament [--context NAME] [--config PATH] source <create|edit|list|discover|delete>
  filament [--context NAME] [--config PATH] sink <create|edit|list|delete>
  filament [--context NAME] [--config PATH] pipeline <create|edit|list|delete>
  filament [--context NAME] [--config PATH] config <path|validate|edit>
  filament [--context NAME] [--config PATH] run <pipeline> [flags]
  filament run --source-connector NAME --sink-connector NAME [flags]
  filament context <list|current|use>

Use --help after a command or operation for its flags. Connector fields always
use --source-<field> or --sink-<field>. Secret fields accept plaintext values,
quoted $NAME or ${NAME} references, or --<kind>-<field>-env VARIABLE. Editing
commands can remove a field with --unset <kind>-<field>.
`

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

func (o *helpOutput) println(value string) {
	if o.err == nil {
		_, o.err = fmt.Fprintln(o.w, value)
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

func (a *cliApp) printConnectionHelp(kind string) error {
	discover := ""
	if kind == "source" {
		discover = fmt.Sprintf("  filament %s discover [name] [flags]\n", kind)
	}
	out := &helpOutput{w: a.stdout}
	out.printf(`%s commands

Usage:
  filament %s create <name> --%s-connector NAME [flags]
  filament %s edit <name> [flags] [--unset %s-FIELD]
  filament %s list
%s  filament %s delete <name> [--force]

Use --help with create, edit, or discover and a connector selection to list
the connector-derived flags.
`, strings.ToUpper(kind[:1])+kind[1:], kind, kind, kind, kind, kind, discover, kind)
	return out.err
}

func (a *cliApp) printConnectionOperationHelp(kind, operation string, args []string, doc climodel.Document) error {
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

	switch operation {
	case "create":
		out.printf("Usage: filament %s create <name> --%sconnector NAME [flags]\n", kind, prefix)
	case "edit":
		out.printf("Usage: filament %s edit <name> [--%sconnector NAME] [flags] [--unset %sFIELD]\n", kind, prefix, prefix)
	case "discover":
		out.println("Usage: filament source discover [name] [--source-connector NAME] [flags]")
	case "list":
		out.printf("Usage: filament %s list\n", kind)
	case "delete":
		out.printf("Usage: filament %s delete <name> [--force]\n", kind)
	default:
		return a.printConnectionHelp(kind)
	}

	if connectorName == "" {
		out.printf("\nAvailable %s connectors: %s\n", kind, strings.Join(a.connectorNames(kind), ", "))
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
	out.printf("\n%s connector flags:\n", connectorName)
	printSchemaFlags(out, prefix, schema, scope, operation == "discover" && name == "")
	return out.err
}

func (a *cliApp) printRunHelp(ctx context.Context, args []string) error {
	out := &helpOutput{w: a.stdout}
	out.print(runHelp)
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
		out.printf("\n%s %s flags:\n", name, kind)
		printSchemaFlags(out, kind+"-", schema, filament.ScopeConnection, true)
		printed[kind] = true
	}
	parsed, _ := a.parseCommandArgs(removeHelp(args))
	if name := firstPositional(parsed); name != "" {
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
						out.printf("\n%s %s flags:\n", conn.Type, kind)
						printSchemaFlags(out, kind+"-", schema, filament.ScopePipeline, false)
					}
				}
			}
		}
	}
	return out.err
}

func (a *cliApp) printPipelineOperationHelp(_ string, args []string, doc climodel.Document) error {
	out := &helpOutput{w: a.stdout}
	out.print(pipelineHelp)
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
		out.printf("\n%s %s flags:\n", conn.Type, kind)
		printSchemaFlags(out, kind+"-", schema, filament.ScopePipeline, false)
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

func printSchemaFlags(out *helpOutput, prefix string, schema filament.ConfigSchema, scope filament.FieldScope, allScopes bool) {
	for _, field := range schema.Fields {
		actualScope := field.Scope
		if actualScope == filament.ScopeUnspecified {
			actualScope = filament.ScopeConnection
		}
		if !allScopes && actualScope != scope {
			continue
		}
		name := "--" + prefix + strings.ReplaceAll(field.Name, "_", "-")
		required := ""
		if field.Required && field.Default == nil {
			required = " (required)"
		}
		out.printf("  %-32s %s%s\n", name+" "+fieldTypeName(field.Type), field.Help, required)
		if cliapp.IsSecretField(field) {
			out.printf("  %-32s environment-variable reference (NAME, $NAME, or ${NAME})\n", name+"-env VARIABLE")
		}
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

const pipelineHelp = `Pipeline commands

Usage:
  filament pipeline create <name> --source NAME --sink NAME [--resources LIST] [flags]
  filament pipeline edit <name> [flags] [--unset source-FIELD|sink-FIELD]
  filament pipeline list
  filament pipeline delete <name> [--force]

An omitted --resources selection means all resources discovered by the source.
Connector pipeline fields use --source-<field> and --sink-<field>.
Repeat --unset to remove optional connector fields from an existing pipeline.
`

const configHelp = `Configuration commands

Usage:
  filament config path
  filament config validate
  filament config edit
`

const runHelp = `Run a saved pipeline or an inline source-to-sink transfer.

Usage:
  filament run <pipeline> [--resources LIST] [--sync-mode full] [--write-mode MODE] [--unset KIND-FIELD]
  filament run --source-connector NAME --sink-connector NAME [flags]

An omitted --resources selection means all resources discovered by the source.
Use --source-<field> and --sink-<field> for connector configuration.
`
