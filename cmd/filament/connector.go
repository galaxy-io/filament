package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/galaxy-io/filament"
)

func (a *cliApp) runConnectionCommand(ctx context.Context, kind string, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.printConnectionHelp(kind)
		return nil
	}
	if helpRequested(args[1:]) && rawFlagValue(args[1:], kind+"-connector") != "" {
		a.printConnectionOperationHelp(kind, args[0], args[1:], newDocument())
		return nil
	}
	store := configStore{path: a.configPath}
	doc, _, err := store.load()
	if err != nil {
		return err
	}
	if helpRequested(args[1:]) {
		a.printConnectionOperationHelp(kind, args[0], args[1:], doc)
		return nil
	}
	switch args[0] {
	case "discover":
		if kind != "source" {
			return fmt.Errorf("discover is only available for sources")
		}
		return a.discoverSource(ctx, args[1:], doc)
	case "list":
		if len(args) != 1 {
			return fmt.Errorf("usage: filament %s list", kind)
		}
		return a.listConnections(kind, doc)
	case "create", "edit":
		return a.changeConnection(args[0], kind, args[1:], doc, store)
	case "delete":
		return a.deleteConnection(kind, args[1:], doc, store)
	default:
		return fmt.Errorf("unknown %s operation %q", kind, args[0])
	}
}

func (a *cliApp) changeConnection(operation, kind string, args []string, doc configDocument, store configStore) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	var existing *connection
	if operation == "edit" {
		if name == "" {
			return fmt.Errorf("usage: filament %s edit <name> [flags]", kind)
		}
		current, ok := connectionMap(kind, doc)[name]
		if !ok {
			return fmt.Errorf("%s %q does not exist", kind, name)
		}
		existing = &current
	} else if name != "" {
		if _, duplicate := connectionMap(kind, doc)[name]; duplicate {
			return fmt.Errorf("%s %q already exists", kind, name)
		}
	}
	if name == "" {
		return fmt.Errorf("%s name is required; example: filament %s %s production --%s-connector postgres --%s-dsn-env POSTGRES_DSN", kind, kind, operation, kind, kind)
	}
	conn, err := a.connectionFromFlags(kind, name, existing, parsed.flags)
	if err != nil {
		return err
	}
	testDoc := doc
	if kind == "source" {
		testDoc.Sources = cloneMap(doc.Sources)
		testDoc.Sources[name] = conn
	} else {
		testDoc.Sinks = cloneMap(doc.Sinks)
		testDoc.Sinks[name] = conn
	}
	if err := validateDocument(testDoc, a.catalog); err != nil {
		return err
	}
	if err := store.put(kind+"s", name, conn); err != nil {
		return err
	}
	printSuccess(a.statusWriter(), fmt.Sprintf("%s %s %s", pastTense(operation), name, kind))
	return nil
}

func (a *cliApp) deleteConnection(kind string, args []string, doc configDocument, store configStore) error {
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
	if err := ensureConnectionUnreferenced(kind, name, doc); err != nil {
		return err
	}
	if _, ok := connectionMap(kind, doc)[name]; !ok {
		return fmt.Errorf("%s %q does not exist", kind, name)
	}
	if !force {
		confirmed, err := a.confirmDelete(kind, name)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := store.delete(kind+"s", name); err != nil {
		return err
	}
	fmt.Fprintf(a.statusWriter(), "Deleted %s %q.\n", kind, name)
	return nil
}

func (a *cliApp) connectionFromFlags(kind, name string, existing *connection, flags map[string][]string) (connection, error) {
	conn := connection{Config: map[string]any{}}
	if existing != nil {
		conn = *existing
		conn.Config = cloneMap(existing.Config)
	}
	prefix := kind + "-"
	connectorFlag := prefix + "connector"
	typeName := lastFlag(flags, connectorFlag)
	if typeName == "" {
		typeName = conn.Type
	}
	if typeName == "" {
		return conn, fmt.Errorf("%s %q: --%s is required", kind, name, connectorFlag)
	}
	conn.Type = typeName
	if existing != nil && existing.Type != typeName {
		conn.Config = map[string]any{}
	}
	var schema filament.ConfigSchema
	if kind == "source" {
		spec, ok := a.catalog.sources[typeName]
		if !ok {
			return conn, fmt.Errorf("unknown source connector %q", typeName)
		}
		schema = spec.Config
	} else {
		spec, ok := a.catalog.sinks[typeName]
		if !ok {
			return conn, fmt.Errorf("unknown sink connector %q", typeName)
		}
		schema = spec.Config
	}
	allowed := map[string]bool{connectorFlag: true}
	for _, field := range orderedFields(schema, filament.ScopeConnection) {
		flagName := prefix + strings.ReplaceAll(field.Name, "_", "-")
		allowed[flagName] = true
		if isSecretField(field) {
			envFlag := flagName + "-env"
			allowed[envFlag] = true
			raw, plaintext := flagValue(flags, flagName)
			envName, fromEnv := flagValue(flags, envFlag)
			if plaintext && fromEnv {
				return conn, fmt.Errorf("--%s and --%s cannot be combined", flagName, envFlag)
			}
			if fromEnv {
				conn.Config[field.Name] = "env:" + strings.TrimPrefix(envName, "env:")
			}
			if plaintext {
				value, err := parseFlagValue(field, raw)
				if err != nil {
					return conn, fmt.Errorf("--%s: %w", flagName, err)
				}
				conn.Config[field.Name] = value
			}
			continue
		}
		if raw, present := flagValue(flags, flagName); present {
			value, err := parseFlagValue(field, raw)
			if err != nil {
				return conn, fmt.Errorf("--%s: %w", flagName, err)
			}
			conn.Config[field.Name] = value
		}
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return conn, err
	}
	return conn, nil
}

func (a *cliApp) listConnections(kind string, doc configDocument) error {
	connections := connectionMap(kind, doc)
	if len(connections) == 0 {
		fmt.Fprintf(a.stdout, "No saved %ss.\n", kind)
		return nil
	}
	table := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "Name\tConnector\tDescription")
	for _, name := range sortedKeys(connections) {
		conn := connections[name]
		description := ""
		if kind == "source" {
			if spec, ok := a.catalog.sources[conn.Type]; ok {
				description = spec.Description
			}
		} else if spec, ok := a.catalog.sinks[conn.Type]; ok {
			description = spec.Description
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", name, conn.Type, description)
	}
	return table.Flush()
}

func connectionMap(kind string, doc configDocument) map[string]connection {
	if kind == "sink" {
		return doc.Sinks
	}
	return doc.Sources
}

func ensureConnectionUnreferenced(kind, name string, doc configDocument) error {
	for pipelineName, p := range doc.Pipelines {
		if (kind == "source" && p.Source.Ref == name) || (kind == "sink" && p.Sink.Ref == name) {
			return fmt.Errorf("%s %q is referenced by pipeline %q; delete or edit that pipeline first", kind, name, pipelineName)
		}
	}
	return nil
}
