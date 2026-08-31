package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	"github.com/galaxy-io/filament/registry"
)

const configVersion = localtarget.ConfigVersion

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type configDocument = localtarget.Document
type connection = localtarget.Connection
type pipeline = localtarget.Pipeline
type pipelineNode = localtarget.PipelineNode

func newDocument() configDocument {
	return localtarget.NewDocument()
}

type connectorCatalog struct {
	sources map[string]filament.ConnectorSpec
	sinks   map[string]filament.SinkSpec
}

func loadCatalog() connectorCatalog {
	c := connectorCatalog{
		sources: map[string]filament.ConnectorSpec{},
		sinks:   map[string]filament.SinkSpec{},
	}
	for _, spec := range registry.DefaultSources.Specs() {
		c.sources[spec.Name] = spec
	}
	for _, spec := range registry.DefaultSinks.Specs() {
		c.sinks[spec.Name] = spec
	}
	return c
}

func (c connectorCatalog) sourceNames() []string { return sortedKeys(c.sources) }
func (c connectorCatalog) sinkNames() []string   { return sortedKeys(c.sinks) }

// Description returns connector help text for a source or sink connector.
func (c connectorCatalog) Description(kind, connector string) (string, bool) {
	if kind == "sink" {
		spec, ok := c.sinks[connector]
		return spec.Description, ok
	}
	spec, ok := c.sources[connector]
	return spec.Description, ok
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validateDocument(d configDocument, catalog connectorCatalog) error {
	if d.Version != configVersion {
		return fmt.Errorf("version: got %d, want %d", d.Version, configVersion)
	}
	for _, name := range sortedKeys(d.Sources) {
		if err := validateName("source", name); err != nil {
			return err
		}
		conn := d.Sources[name]
		spec, ok := catalog.sources[conn.Type]
		if !ok {
			return fmt.Errorf("source %q: unknown connector %q", name, conn.Type)
		}
		if err := validateConnectionFields("source "+name, spec.Config, conn); err != nil {
			return err
		}
	}
	for _, name := range sortedKeys(d.Sinks) {
		if err := validateName("sink", name); err != nil {
			return err
		}
		conn := d.Sinks[name]
		spec, ok := catalog.sinks[conn.Type]
		if !ok {
			return fmt.Errorf("sink %q: unknown connector %q", name, conn.Type)
		}
		if err := validateConnectionFields("sink "+name, spec.Config, conn); err != nil {
			return err
		}
	}
	for _, name := range sortedKeys(d.Pipelines) {
		if err := validateName("pipeline", name); err != nil {
			return err
		}
		p := d.Pipelines[name]
		source, ok := d.Sources[p.Source.Ref]
		if !ok {
			return fmt.Errorf("pipeline %q: source ref %q does not exist", name, p.Source.Ref)
		}
		sink, ok := d.Sinks[p.Sink.Ref]
		if !ok {
			return fmt.Errorf("pipeline %q: sink ref %q does not exist", name, p.Sink.Ref)
		}
		for _, resource := range p.Resources {
			if strings.TrimSpace(resource) == "" {
				return fmt.Errorf("pipeline %q: resources contains an empty name", name)
			}
		}
		if p.SyncMode != "full" {
			return fmt.Errorf("pipeline %q: sync_mode %q is not available for local runs; use full", name, p.SyncMode)
		}
		write := filament.WriteMode(p.WriteMode)
		if !containsWriteMode(filament.WriteModesFor(filament.ModeFull), write) {
			return fmt.Errorf("pipeline %q: write_mode %q is not compatible with full reads", name, p.WriteMode)
		}
		sinkSpec := catalog.sinks[sink.Type]
		if !sinkSupports(sinkSpec, write) {
			return fmt.Errorf("pipeline %q: sink %q does not support write_mode %q", name, p.Sink.Ref, write)
		}
		if err := validateScopedFields("pipeline "+name+" source", catalog.sources[source.Type].Config, p.Source.Config, filament.ScopePipeline); err != nil {
			return err
		}
		if err := validateScopedFields("pipeline "+name+" sink", sinkSpec.Config, p.Sink.Config, filament.ScopePipeline); err != nil {
			return err
		}
	}
	return nil
}

func validateName(kind, name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%s name %q must match %s", kind, name, namePattern)
	}
	return nil
}

func validateConnectionFields(label string, schema filament.ConfigSchema, conn connection) error {
	return validateScopedFields(label, schema, conn.Config, filament.ScopeConnection)
}

func validateScopedFields(label string, schema filament.ConfigSchema, values map[string]any, scope filament.FieldScope) error {
	fields := orderedFields(schema, scope)
	canonical := cloneConfigMap(values)
	if scope == filament.ScopeConnection {
		canonical = canonicalizeConfig(schema, canonical)
	} else {
		pruneInactiveFields(fields, canonical)
	}
	return validateFields(label, fields, canonical)
}

func validateFields(label string, fields []filament.ConfigField, values map[string]any) error {
	effective := valuesWithDefaults(fields, values)
	known := make(map[string]bool, len(fields))
	visible := make(map[string]filament.ConfigField, len(fields))
	for _, field := range fields {
		known[field.Name] = true
		if fieldVisible(field, effective) {
			visible[field.Name] = field
		}
	}
	for name, value := range values {
		field, ok := visible[name]
		if !ok {
			if known[name] {
				return fmt.Errorf("%s: field %q is not active for the selected configuration", label, name)
			}
			return fmt.Errorf("%s: unknown or misplaced field %q", label, name)
		}
		if err := validateFieldValue(field, value); err != nil {
			return fmt.Errorf("%s: field %q: %w", label, name, err)
		}
		if isSecretField(field) {
			text, _ := value.(string)
			if strings.HasPrefix(text, "env:") {
				if _, valid := environmentReferenceName(text); !valid {
					return fmt.Errorf("%s: field %q environment reference must use env:VARIABLE", label, name)
				}
			}
		}
		if len(field.Fields) > 0 {
			nested, _ := value.(map[string]any)
			if err := validateFields(label+" "+name, field.Fields, nested); err != nil {
				return err
			}
		}
	}
	requiredSeen := map[string]bool{}
	for _, field := range fields {
		if requiredSeen[field.Name] || !fieldVisible(field, effective) {
			continue
		}
		requiredSeen[field.Name] = true
		if !field.Required || field.Default != nil {
			continue
		}
		if value, ok := values[field.Name]; !ok || isEmpty(value) {
			return fmt.Errorf("%s: field %q is required", label, field.Name)
		}
	}
	return nil
}

func fieldVisible(field filament.ConfigField, values map[string]any) bool {
	if field.VisibleWhen == nil {
		return true
	}
	actual := fmt.Sprint(values[field.VisibleWhen.Field])
	for _, value := range field.VisibleWhen.Values {
		if actual == value {
			return true
		}
	}
	return false
}

func validateFieldValue(field filament.ConfigField, value any) error {
	switch field.Type {
	case filament.FieldString, filament.FieldSecret, filament.FieldEnum, filament.FieldDuration:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if field.Type == filament.FieldDuration && s != "" {
			if _, err := time.ParseDuration(s); err != nil {
				return fmt.Errorf("must be a duration such as 30s or 5m")
			}
		}
		if field.Type == filament.FieldEnum && s != "" {
			found := false
			for _, option := range field.Enum {
				found = found || option.Value == s
			}
			if !found {
				return fmt.Errorf("must be one of %s", strings.Join(enumValues(field.Enum), ", "))
			}
		}
	case filament.FieldInt:
		switch value.(type) {
		case int, int64, uint64:
		default:
			return fmt.Errorf("must be an integer")
		}
	case filament.FieldBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case filament.FieldObject:
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("must be an object")
		}
	case filament.FieldList:
		var values []string
		switch typed := value.(type) {
		case []string:
			values = typed
		case []any:
			for _, item := range typed {
				text, ok := item.(string)
				if !ok {
					return fmt.Errorf("must be a list of strings")
				}
				values = append(values, text)
			}
		default:
			return fmt.Errorf("must be a list of strings")
		}
		if len(field.Enum) > 0 {
			allowed := enumValues(field.Enum)
			for _, value := range values {
				if !containsString(allowed, value) {
					return fmt.Errorf("items must be one of %s", strings.Join(allowed, ", "))
				}
			}
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func enumValues(options []filament.EnumOption) []string {
	values := make([]string, len(options))
	for i, option := range options {
		values[i] = option.Value
	}
	return values
}

func isSecretField(field filament.ConfigField) bool {
	return field.Type == filament.FieldSecret || field.Secret
}

func isEmpty(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func containsWriteMode(modes []filament.WriteMode, wanted filament.WriteMode) bool {
	for _, mode := range modes {
		if mode == wanted {
			return true
		}
	}
	return false
}

func sinkSupports(spec filament.SinkSpec, wanted filament.WriteMode) bool {
	for _, capability := range spec.Capabilities.WritePolicies {
		if capability.Mode == wanted {
			return true
		}
	}
	return false
}
