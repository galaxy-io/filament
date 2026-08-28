package main

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// canonicalizeConfig removes inactive conditional branches without
// materializing schema defaults. Callers must explicitly select any non-default
// branch, such as connection_method=url for a database DSN.
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
		if fieldVisible(field, effective) {
			visible[field.Name] = field
		}
	}
	for name := range values {
		_, isVisible := visible[name]
		if known[name] && !isVisible {
			delete(values, name)
		}
	}
	for name, field := range visible {
		nested, ok := values[name].(map[string]any)
		if ok && len(field.Fields) > 0 {
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
		for i, item := range typed {
			result[i] = cloneConfigValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func normalizeSavedSecretReferences(schema filament.ConfigSchema, values map[string]any) (map[string]any, error) {
	result := cloneConfigMap(values)
	if err := transformSecretFields(schema.Fields, result, "", false); err != nil {
		return nil, err
	}
	return result, nil
}

func resolveConfigSecretReferences(schema filament.ConfigSchema, values map[string]any) (map[string]any, error) {
	result := cloneConfigMap(values)
	if err := transformSecretFields(schema.Fields, result, "", true); err != nil {
		return nil, err
	}
	return result, nil
}

func transformSecretFields(fields []filament.ConfigField, values map[string]any, parent string, resolve bool) error {
	for _, field := range fields {
		value, present := values[field.Name]
		if !present {
			continue
		}
		path := joinConfigPath(parent, field.Name)
		if isSecretField(field) {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("field %q must be a string", path)
			}
			if resolve {
				resolved, err := resolveEnvironmentReference(text)
				if err != nil {
					return fmt.Errorf("field %q: %w", path, err)
				}
				values[field.Name] = resolved
				continue
			}
			if name, referenced := environmentReferenceName(text); referenced {
				values[field.Name] = "env:" + name
			} else if strings.HasPrefix(text, "env:") {
				return fmt.Errorf("field %q environment reference must use env:NAME", path)
			}
			continue
		}
		nested, ok := value.(map[string]any)
		if ok && len(field.Fields) > 0 {
			if err := transformSecretFields(field.Fields, nested, path, resolve); err != nil {
				return err
			}
		}
	}
	return nil
}

func joinConfigPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}
