package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

const configVersion = 1

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envRefPattern  = regexp.MustCompile(`^env:[A-Za-z_][A-Za-z0-9_]*$`)
)

type configDocument struct {
	Version   int                   `yaml:"version"`
	Sources   map[string]connection `yaml:"sources,omitempty"`
	Sinks     map[string]connection `yaml:"sinks,omitempty"`
	Pipelines map[string]pipeline   `yaml:"pipelines,omitempty"`
}

type connection struct {
	Type   string         `yaml:"type"`
	Config map[string]any `yaml:"config,omitempty"`
}

type pipeline struct {
	Source    pipelineNode `yaml:"source"`
	Sink      pipelineNode `yaml:"sink"`
	Resources []string     `yaml:"resources,omitempty"`
	SyncMode  string       `yaml:"sync_mode"`
	WriteMode string       `yaml:"write_mode"`
}

type pipelineNode struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

func newDocument() configDocument {
	return configDocument{
		Version:   configVersion,
		Sources:   map[string]connection{},
		Sinks:     map[string]connection{},
		Pipelines: map[string]pipeline{},
	}
}

func (d *configDocument) normalize() {
	if d.Sources == nil {
		d.Sources = map[string]connection{}
	}
	if d.Sinks == nil {
		d.Sinks = map[string]connection{}
	}
	if d.Pipelines == nil {
		d.Pipelines = map[string]pipeline{}
	}
	for name, p := range d.Pipelines {
		if p.SyncMode == "" {
			p.SyncMode = "full"
		}
		if p.WriteMode == "" {
			p.WriteMode = "replace"
		}
		d.Pipelines[name] = p
	}
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
	if err := validateScopedFields(label, schema, conn.Config, filament.ScopeConnection); err != nil {
		return err
	}
	fields := fieldsForScope(schema, filament.ScopeConnection)
	for name, field := range fields {
		if !isSecretField(field) {
			continue
		}
		value, _ := conn.Config[name].(string)
		if strings.HasPrefix(value, "env:") && !envRefPattern.MatchString(value) {
			return fmt.Errorf("%s: config.%s must use env:VARIABLE", label, name)
		}
	}
	return nil
}

func validateScopedFields(label string, schema filament.ConfigSchema, values map[string]any, scope filament.FieldScope) error {
	fields := fieldsForScope(schema, scope)
	for name, value := range values {
		field, ok := fields[name]
		if !ok {
			return fmt.Errorf("%s: unknown or misplaced field %q", label, name)
		}
		if err := validateFieldValue(field, value); err != nil {
			return fmt.Errorf("%s: field %q: %w", label, name, err)
		}
	}
	for name, field := range fields {
		if !field.Required || field.Default != nil || !fieldVisible(field, values) {
			continue
		}
		if value, ok := values[name]; !ok || isEmpty(value) {
			return fmt.Errorf("%s: field %q is required", label, name)
		}
	}
	return nil
}

func fieldsForScope(schema filament.ConfigSchema, scope filament.FieldScope) map[string]filament.ConfigField {
	fields := map[string]filament.ConfigField{}
	for _, field := range schema.Fields {
		fieldScope := field.Scope
		if fieldScope == filament.ScopeUnspecified {
			fieldScope = filament.ScopeConnection
		}
		if fieldScope == scope {
			fields[field.Name] = field
		}
	}
	return fields
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
	return value == nil || value == ""
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
