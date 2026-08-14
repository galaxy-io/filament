package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/galaxy-io/filament"
)

var rootHelp = `Filament local CLI

Usage:
  filament [--config PATH] source <create|edit|list|discover|delete>
  filament [--config PATH] sink <create|edit|list|delete>
  filament [--config PATH] pipeline <create|edit|list|delete>
  filament [--config PATH] config <path|validate|edit>
  filament [--config PATH] run <pipeline> [flags]
  filament run --source-connector NAME --sink-connector NAME [flags]

Use --help after a command or operation for its flags. Connector fields always
use --source-<field> or --sink-<field>. Secret fields accept plaintext values
or the corresponding --<kind>-<field>-env VARIABLE form.
`

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

func (a *cliApp) printConnectionHelp(kind string) {
	discover := ""
	if kind == "source" {
		discover = fmt.Sprintf("  filament %s discover [name] [flags]\n", kind)
	}
	fmt.Fprintf(a.stdout, `%s commands

Usage:
  filament %s create <name> --%s-connector NAME [flags]
  filament %s edit <name> [flags]
  filament %s list
%s  filament %s delete <name> [--force]

Use --help with create, edit, or discover and a connector selection to list
the connector-derived flags.
`, strings.ToUpper(kind[:1])+kind[1:], kind, kind, kind, kind, discover, kind)
}

func (a *cliApp) printConnectionOperationHelp(kind, operation string, args []string, doc configDocument) {
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
		fmt.Fprintf(a.stdout, "Usage: filament %s create <name> --%sconnector NAME [flags]\n", kind, prefix)
	case "edit":
		fmt.Fprintf(a.stdout, "Usage: filament %s edit <name> [--%sconnector NAME] [flags]\n", kind, prefix)
	case "discover":
		fmt.Fprintln(a.stdout, "Usage: filament source discover [name] [--source-connector NAME] [flags]")
	case "list":
		fmt.Fprintf(a.stdout, "Usage: filament %s list\n", kind)
	case "delete":
		fmt.Fprintf(a.stdout, "Usage: filament %s delete <name> [--force]\n", kind)
	default:
		a.printConnectionHelp(kind)
		return
	}

	if connectorName == "" {
		fmt.Fprintf(a.stdout, "\nAvailable %s connectors: %s\n", kind, strings.Join(a.connectorNames(kind), ", "))
		return
	}
	schema, ok := a.connectorSchema(kind, connectorName)
	if !ok {
		fmt.Fprintf(a.stdout, "\nUnknown %s connector %q.\n", kind, connectorName)
		return
	}
	scope := filament.ScopeConnection
	if operation == "discover" && name != "" {
		scope = filament.ScopePipeline
	}
	fmt.Fprintf(a.stdout, "\n%s connector flags:\n", connectorName)
	printSchemaFlags(a.stdout, prefix, schema, scope, operation == "discover" && name == "")
}

func (a *cliApp) printRunHelp(args []string) {
	fmt.Fprint(a.stdout, runHelp)
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
		fmt.Fprintf(a.stdout, "\n%s %s flags:\n", name, kind)
		printSchemaFlags(a.stdout, kind+"-", schema, filament.ScopeConnection, true)
		printed[kind] = true
	}
	parsed, _ := a.parseCommandArgs(removeHelp(args))
	if name := firstPositional(parsed); name != "" {
		if doc, _, err := (configStore{path: a.configPath}).load(); err == nil {
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
						fmt.Fprintf(a.stdout, "\n%s %s flags:\n", conn.Type, kind)
						printSchemaFlags(a.stdout, kind+"-", schema, filament.ScopePipeline, false)
					}
				}
			}
		}
	}
}

func (a *cliApp) printPipelineOperationHelp(_ string, args []string, doc configDocument) {
	fmt.Fprint(a.stdout, pipelineHelp)
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
		fmt.Fprintf(a.stdout, "\n%s %s flags:\n", conn.Type, kind)
		printSchemaFlags(a.stdout, kind+"-", schema, filament.ScopePipeline, false)
	}
}

func (a *cliApp) connectorNames(kind string) []string {
	if kind == "sink" {
		return a.catalog.sinkNames()
	}
	return a.catalog.sourceNames()
}

func (a *cliApp) connectorSchema(kind, name string) (filament.ConfigSchema, bool) {
	if kind == "sink" {
		spec, ok := a.catalog.sinks[name]
		return spec.Config, ok
	}
	spec, ok := a.catalog.sources[name]
	return spec.Config, ok
}

func printSchemaFlags(w io.Writer, prefix string, schema filament.ConfigSchema, scope filament.FieldScope, allScopes bool) {
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
		fmt.Fprintf(w, "  %-32s %s%s\n", name+" "+fieldTypeName(field.Type), field.Help, required)
		if isSecretField(field) {
			fmt.Fprintf(w, "  %-32s environment-variable reference\n", name+"-env VARIABLE")
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
  filament pipeline edit <name> [flags]
  filament pipeline list
  filament pipeline delete <name> [--force]

An omitted --resources selection means all resources discovered by the source.
Connector pipeline fields use --source-<field> and --sink-<field>.
`

const configHelp = `Configuration commands

Usage:
  filament config path
  filament config validate
  filament config edit
`

const runHelp = `Run a saved pipeline or an inline source-to-sink transfer.

Usage:
  filament run <pipeline> [--resources LIST] [--sync-mode full] [--write-mode MODE]
  filament run --source-connector NAME --sink-connector NAME [flags]

An omitted --resources selection means all resources discovered by the source.
Use --source-<field> and --sink-<field> for connector configuration.
`
