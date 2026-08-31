package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	textoutput "github.com/galaxy-io/filament/cmd/internal/cli/output/text"
)

func (a *cliApp) runConnectionCommand(ctx context.Context, kind string, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return a.printConnectionHelp(kind)
	}
	if helpRequested(args[1:]) && rawFlagValue(args[1:], kind+"-connector") != "" {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], newDocument())
	}
	if args[0] == "list" && helpRequested(args[1:]) {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], newDocument())
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return fmt.Errorf("usage: filament %s list", kind)
		}
		result, err := a.queries.Connections(ctx, kind)
		if err != nil {
			return err
		}
		return textoutput.Connections(a.stdout, result)
	}
	store := configStore{path: a.configPath}
	doc, _, err := store.load()
	if err != nil {
		return err
	}
	if helpRequested(args[1:]) {
		return a.printConnectionOperationHelp(kind, args[0], args[1:], doc)
	}
	switch args[0] {
	case "discover":
		if kind != "source" {
			return fmt.Errorf("discover is only available for sources")
		}
		return a.discoverSource(ctx, args[1:], doc)
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
	return printSuccess(a.statusWriter(), fmt.Sprintf("%s %s %s", pastTense(operation), name, kind))
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
	_, err = fmt.Fprintf(a.statusWriter(), "Deleted %s %q.\n", kind, name)
	return err
}

func (a *cliApp) connectionFromFlags(kind, name string, existing *connection, flags map[string][]string) (connection, error) {
	conn, prefix, err := initializeConnection(kind, name, existing, flags)
	if err != nil {
		return conn, err
	}
	connectorFlag := prefix + "connector"
	schema, err := a.connectionSchema(kind, conn.Type)
	if err != nil {
		return conn, err
	}
	allowed := map[string]bool{connectorFlag: true}
	unsetTargets := map[string]func(){}
	for _, field := range orderedFields(schema, filament.ScopeConnection) {
		flagName := prefix + strings.ReplaceAll(field.Name, "_", "-")
		allowed[flagName] = true
		fieldName := field.Name
		unsetTargets[flagName] = func() { delete(conn.Config, fieldName) }
		if isSecretField(field) {
			envFlag := flagName + "-env"
			allowed[envFlag] = true
			raw, plaintext := flagValue(flags, flagName)
			envName, fromEnv := flagValue(flags, envFlag)
			if plaintext && fromEnv {
				return conn, fmt.Errorf("--%s and --%s cannot be combined", flagName, envFlag)
			}
			if fromEnv {
				name, err := normalizeEnvironmentName(envName)
				if err != nil {
					return conn, fmt.Errorf("--%s: %w", envFlag, err)
				}
				conn.Config[field.Name] = "env:" + name
			}
			if plaintext {
				if name, referenced := environmentReferenceName(raw); referenced {
					conn.Config[field.Name] = "env:" + name
					continue
				}
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
	if err := applyFieldUnsets(flags, unsetTargets, allowed); err != nil {
		return conn, err
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return conn, err
	}
	conn.Config = canonicalizeConfig(schema, conn.Config)
	normalized, err := normalizeSavedSecretReferences(schema, conn.Config)
	if err != nil {
		return conn, fmt.Errorf("%s %q: %w", kind, name, err)
	}
	conn.Config = normalized
	return conn, nil
}

func initializeConnection(kind, name string, existing *connection, flags map[string][]string) (connection, string, error) {
	conn := connection{Config: map[string]any{}}
	if existing != nil {
		conn = *existing
		conn.Config = cloneConfigMap(existing.Config)
	}
	prefix := kind + "-"
	connectorFlag := prefix + "connector"
	typeName := lastFlag(flags, connectorFlag)
	if typeName == "" {
		typeName = conn.Type
	}
	if typeName == "" {
		return conn, prefix, fmt.Errorf("%s %q: --%s is required", kind, name, connectorFlag)
	}
	conn.Type = typeName
	if existing != nil && existing.Type != typeName {
		conn.Config = map[string]any{}
	}
	return conn, prefix, nil
}

func (a *cliApp) connectionSchema(kind, typeName string) (filament.ConfigSchema, error) {
	if kind == "source" {
		spec, ok := a.catalog.sources[typeName]
		if !ok {
			return filament.ConfigSchema{}, fmt.Errorf("unknown source connector %q", typeName)
		}
		return spec.Config, nil
	}
	spec, ok := a.catalog.sinks[typeName]
	if !ok {
		return filament.ConfigSchema{}, fmt.Errorf("unknown sink connector %q", typeName)
	}
	return spec.Config, nil
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
