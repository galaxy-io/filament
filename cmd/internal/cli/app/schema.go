package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// ValidateDocument validates a structured configuration against a target's
// connector catalog.
func ValidateDocument(document model.Document, catalog model.Catalog) error {
	if document.Version != model.ConfigVersion {
		return fmt.Errorf("version: got %d, want %d", document.Version, model.ConfigVersion)
	}
	for _, name := range sortedKeys(document.Sources) {
		if err := validateName("source", name); err != nil {
			return err
		}
		connection := document.Sources[name]
		spec, ok := catalog.Sources[connection.Type]
		if !ok {
			return fmt.Errorf("source %q: unknown connector %q", name, connection.Type)
		}
		if err := validateConnectionFields("source "+name, spec.Config, connection); err != nil {
			return err
		}
	}
	for _, name := range sortedKeys(document.Sinks) {
		if err := validateName("sink", name); err != nil {
			return err
		}
		connection := document.Sinks[name]
		spec, ok := catalog.Sinks[connection.Type]
		if !ok {
			return fmt.Errorf("sink %q: unknown connector %q", name, connection.Type)
		}
		if err := validateConnectionFields("sink "+name, spec.Config, connection); err != nil {
			return err
		}
	}
	for _, name := range sortedKeys(document.Pipelines) {
		if err := validateName("pipeline", name); err != nil {
			return err
		}
		pipeline := document.Pipelines[name]
		source, sourceOK := document.Sources[pipeline.Source.Ref]
		if !sourceOK {
			return fmt.Errorf("pipeline %q: source ref %q does not exist", name, pipeline.Source.Ref)
		}
		sink, sinkOK := document.Sinks[pipeline.Sink.Ref]
		if !sinkOK {
			return fmt.Errorf("pipeline %q: sink ref %q does not exist", name, pipeline.Sink.Ref)
		}
		for _, resource := range pipeline.Resources {
			if strings.TrimSpace(resource) == "" {
				return fmt.Errorf("pipeline %q: resources contains an empty name", name)
			}
		}
		if pipeline.SyncMode != "full" {
			return fmt.Errorf("pipeline %q: sync_mode %q is not supported; use full", name, pipeline.SyncMode)
		}
		writeMode := filament.WriteMode(pipeline.WriteMode)
		if !containsWriteMode(filament.WriteModesFor(filament.ModeFull), writeMode) {
			return fmt.Errorf("pipeline %q: write_mode %q is not compatible with full reads", name, pipeline.WriteMode)
		}
		sinkSpec := catalog.Sinks[sink.Type]
		if !SinkSupports(sinkSpec, writeMode) {
			return fmt.Errorf("pipeline %q: sink %q does not support write_mode %q", name, pipeline.Sink.Ref, writeMode)
		}
		if err := validateScopedFields("pipeline "+name+" source", catalog.Sources[source.Type].Config, pipeline.Source.Config, filament.ScopePipeline); err != nil {
			return err
		}
		if err := validateScopedFields("pipeline "+name+" sink", sinkSpec.Config, pipeline.Sink.Config, filament.ScopePipeline); err != nil {
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

func validateConnectionFields(label string, schema filament.ConfigSchema, connection model.Connection) error {
	return validateScopedFields(label, schema, connection.Config, filament.ScopeConnection)
}

func validateScopedFields(label string, schema filament.ConfigSchema, values map[string]any, scope filament.FieldScope) error {
	fields := OrderedFields(schema, scope)
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
		if FieldVisible(field, effective) {
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
		if IsSecretField(field) {
			text, _ := value.(string)
			if strings.HasPrefix(text, "env:") {
				if _, valid := EnvironmentReferenceName(text); !valid {
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
		if requiredSeen[field.Name] || !FieldVisible(field, effective) {
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

// OrderedFields selects schema fields belonging to a configuration scope.
func OrderedFields(schema filament.ConfigSchema, scope filament.FieldScope) []filament.ConfigField {
	result := []filament.ConfigField{}
	for _, field := range schema.Fields {
		actual := field.Scope
		if actual == filament.ScopeUnspecified {
			actual = filament.ScopeConnection
		}
		if actual == scope {
			result = append(result, field)
		}
	}
	return result
}

// FieldVisible reports whether a conditional field is active for values.
func FieldVisible(field filament.ConfigField, values map[string]any) bool {
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

// ParseFieldValue converts renderer input to the schema field's native type.
func ParseFieldValue(field filament.ConfigField, raw string) (any, error) {
	switch field.Type {
	case filament.FieldInt:
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("must be an integer")
		}
		return value, nil
	case filament.FieldBool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("must be true or false")
		}
		return value, nil
	case filament.FieldObject:
		var value map[string]any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("must be a JSON object")
		}
		return value, nil
	case filament.FieldList:
		return splitComma(raw), nil
	default:
		if err := validateFieldValue(field, raw); err != nil {
			return nil, err
		}
		return raw, nil
	}
}

func validateFieldValue(field filament.ConfigField, value any) error {
	switch field.Type {
	case filament.FieldString, filament.FieldSecret, filament.FieldEnum, filament.FieldDuration:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if field.Type == filament.FieldDuration && text != "" {
			if _, err := time.ParseDuration(text); err != nil {
				return fmt.Errorf("must be a duration such as 30s or 5m")
			}
		}
		if field.Type == filament.FieldEnum && text != "" && !containsString(enumValues(field.Enum), text) {
			return fmt.Errorf("must be one of %s", strings.Join(enumValues(field.Enum), ", "))
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

// IsSecretField reports whether a schema field should be stored by reference.
func IsSecretField(field filament.ConfigField) bool {
	return field.Type == filament.FieldSecret || field.Secret
}

// SinkSupports reports whether a sink accepts a write mode.
func SinkSupports(spec filament.SinkSpec, wanted filament.WriteMode) bool {
	for _, capability := range spec.Capabilities.WritePolicies {
		if capability.Mode == wanted {
			return true
		}
	}
	return false
}

func canonicalizeConfig(schema filament.ConfigSchema, values map[string]any) map[string]any {
	result := cloneConfigMap(values)
	pruneInactiveFields(schema.Fields, result)
	return result
}

func pruneInactiveFields(fields []filament.ConfigField, values map[string]any) {
	effective := valuesWithDefaults(fields, values)
	visible := make(map[string]filament.ConfigField, len(fields))
	known := make(map[string]bool, len(fields))
	for _, field := range fields {
		known[field.Name] = true
		if FieldVisible(field, effective) {
			visible[field.Name] = field
		}
	}
	for name := range values {
		if _, knownField := known[name]; knownField {
			if _, visibleField := visible[name]; !visibleField {
				delete(values, name)
			}
		}
	}
	for name, field := range visible {
		if nested, ok := values[name].(map[string]any); ok && len(field.Fields) > 0 {
			pruneInactiveFields(field.Fields, nested)
		}
	}
}

func valuesWithDefaults(fields []filament.ConfigField, values map[string]any) map[string]any {
	result := cloneConfigMap(values)
	for _, field := range fields {
		if _, present := result[field.Name]; !present && field.Default != nil {
			result[field.Name] = cloneConfigValue(field.Default)
		}
	}
	return result
}

func cloneConfigMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for name, value := range source {
		result[name] = cloneConfigValue(value)
	}
	return result
}

func cloneConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneConfigMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneConfigValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func splitComma(value string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
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
	for index, option := range options {
		values[index] = option.Value
	}
	return values
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
