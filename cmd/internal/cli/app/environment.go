package app

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// EnvironmentReferenceName recognizes Filament's supported environment
// reference spellings and returns the referenced variable name.
func EnvironmentReferenceName(value string) (string, bool) {
	var name string
	switch {
	case strings.HasPrefix(value, "env:"):
		name = strings.TrimPrefix(value, "env:")
	case strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}"):
		name = strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
	case strings.HasPrefix(value, "$"):
		name = strings.TrimPrefix(value, "$")
	default:
		return "", false
	}
	return name, envNamePattern.MatchString(name)
}

// NormalizeEnvironmentName accepts a variable name or reference and returns
// its canonical variable name.
func NormalizeEnvironmentName(value string) (string, error) {
	if name, ok := EnvironmentReferenceName(value); ok {
		return name, nil
	}
	if envNamePattern.MatchString(value) {
		return value, nil
	}
	return "", fmt.Errorf("environment variable must be NAME, $NAME, ${NAME}, or env:NAME")
}

func normalizeSavedSecretReferences(schema filament.ConfigSchema, values map[string]any) (map[string]any, error) {
	result := cloneConfigMap(values)
	if err := transformSecretFields(schema.Fields, result, "", nil); err != nil {
		return nil, err
	}
	return result, nil
}

// ResolveConfigSecrets resolves environment references at the target execution
// boundary. The target supplies the environment lookup because local and
// remote targets do not share an environment.
func ResolveConfigSecrets(schema filament.ConfigSchema, values map[string]any, lookup func(string) (string, bool)) (map[string]any, error) {
	result := cloneConfigMap(values)
	if err := transformSecretFields(schema.Fields, result, "", lookup); err != nil {
		return nil, err
	}
	return result, nil
}

func transformSecretFields(fields []filament.ConfigField, values map[string]any, parent string, lookup func(string) (string, bool)) error {
	for _, field := range fields {
		value, present := values[field.Name]
		if !present {
			continue
		}
		path := joinConfigPath(parent, field.Name)
		if IsSecretField(field) {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("field %q must be a string", path)
			}
			name, referenced := EnvironmentReferenceName(text)
			if lookup == nil {
				if referenced {
					values[field.Name] = "env:" + name
				} else if strings.HasPrefix(text, "env:") {
					return fmt.Errorf("field %q environment reference must use env:NAME", path)
				}
				continue
			}
			if !referenced {
				if strings.HasPrefix(text, "env:") {
					return fmt.Errorf("field %q environment reference must use env:NAME", path)
				}
				continue
			}
			resolved, ok := lookup(name)
			if !ok {
				return fmt.Errorf("field %q: environment variable %s is not set", path, name)
			}
			values[field.Name] = resolved
			continue
		}
		if nested, ok := value.(map[string]any); ok && len(field.Fields) > 0 {
			if err := transformSecretFields(field.Fields, nested, path, lookup); err != nil {
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

// ResolvedConnectionConfig combines saved and scoped configuration, removes
// inactive fields, and canonicalizes secret references. Resolution is left to
// the selected target's execution environment.
func ResolvedConnectionConfig(connection model.Connection, scoped map[string]any, schema filament.ConfigSchema) (map[string]any, error) {
	config := cloneConfigMap(connection.Config)
	for field, value := range scoped {
		config[field] = cloneConfigValue(value)
	}
	config = canonicalizeConfig(schema, config)
	return normalizeSavedSecretReferences(schema, config)
}
