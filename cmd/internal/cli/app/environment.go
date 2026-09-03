package app

import (
	"fmt"
	"maps"
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
	result := model.CloneConfig(values)
	if err := transformSecretFields(schema.Fields, result, "", nil); err != nil {
		return nil, err
	}
	return result, nil
}

// ResolveConfigSecrets resolves environment references at the target execution
// boundary. The target supplies the environment lookup because local and
// remote targets do not share an environment.
func ResolveConfigSecrets(schema filament.ConfigSchema, values map[string]any, lookup func(string) (string, bool)) (map[string]any, error) {
	result := model.CloneConfig(values)
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
	config := model.CloneConfig(connection.Config)
	for field, value := range scoped {
		config[field] = model.CloneConfigValue(value)
	}
	config = canonicalizeConfig(schema, config)
	return normalizeSavedSecretReferences(schema, config)
}

// ConfigWithoutSecretValues returns a copy with every referenced secret path
// removed: the wire shape paired with a secret_refs map. Local YAML keeps its
// env: references in Config.
func ConfigWithoutSecretValues(values map[string]any, refs map[string]string) map[string]any {
	result := model.CloneConfig(values)
	for path := range refs {
		deleteConfigPath(result, strings.Split(path, "."))
	}
	return result
}

// deleteConfigPath removes path from values and reports whether values is
// now empty, so emptied parents are pruned on the way back up.
func deleteConfigPath(values map[string]any, path []string) bool {
	if len(path) == 1 {
		delete(values, path[0])
		return len(values) == 0
	}
	if nested, ok := values[path[0]].(map[string]any); ok && deleteConfigPath(nested, path[1:]) {
		delete(values, path[0])
	}
	return len(values) == 0
}

// storedSecretReferencePrefix marks a deployment-managed secret inside an
// editable config. The value is never shown; an unchanged placeholder keeps
// the reference and is removed again at the wire.
const storedSecretReferencePrefix = "filament-secret-ref:"

// ConfigWithSecretPlaceholders returns editable config that shows each
// referenced secret as a placeholder instead of a value.
func ConfigWithSecretPlaceholders(values map[string]any, refs map[string]string) map[string]any {
	result := model.CloneConfig(values)
	for path, ref := range refs {
		setConfigPath(result, strings.Split(path, "."), storedSecretReferencePrefix+ref)
	}
	return result
}

// UpdateSecretReferences applies a patch's secret-field changes to refs. An
// unchanged placeholder keeps its reference, env:NAME becomes a reference,
// and anything else clears the reference so the value travels as config.
func UpdateSecretReferences(schema filament.ConfigSchema, patch ConfigPatch, current map[string]string) (map[string]string, error) {
	refs := maps.Clone(current)
	if refs == nil {
		refs = map[string]string{}
	}
	for _, name := range patch.Unset {
		deleteSecretRefPrefix(refs, name)
	}
	if err := updateSecretReferenceFields(schema.Fields, patch.Values, refs, current, ""); err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return refs, nil
}

func updateSecretReferenceFields(fields []filament.ConfigField, values map[string]any, refs, original map[string]string, parent string) error {
	for _, field := range fields {
		value, supplied := values[field.Name]
		if !supplied {
			continue
		}
		path := joinConfigPath(parent, field.Name)
		if IsSecretField(field) {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("field %q must be a string", path)
			}
			switch {
			case strings.HasPrefix(text, storedSecretReferencePrefix):
				if original[path] != strings.TrimPrefix(text, storedSecretReferencePrefix) {
					return fmt.Errorf("field %q holds an unknown stored-secret marker", path)
				}
				refs[path] = original[path]
			default:
				if name, referenced := EnvironmentReferenceName(text); referenced {
					refs[path] = name
				} else {
					delete(refs, path)
				}
			}
			continue
		}
		if nested, ok := value.(map[string]any); ok && len(field.Fields) > 0 {
			// An object patch replaces the object, so refs under it go too.
			deleteSecretRefPrefix(refs, path)
			if err := updateSecretReferenceFields(field.Fields, nested, refs, original, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func setConfigPath(values map[string]any, path []string, value any) {
	if len(path) == 1 {
		values[path[0]] = value
		return
	}
	nested, ok := values[path[0]].(map[string]any)
	if !ok {
		nested = map[string]any{}
		values[path[0]] = nested
	}
	setConfigPath(nested, path[1:], value)
}

func deleteSecretRefPrefix(refs map[string]string, prefix string) {
	for path := range refs {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			delete(refs, path)
		}
	}
}
