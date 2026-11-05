package compile

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// scopeByField indexes a schema's fields by name -> scope. Fields left
// unspecified are reported as ScopeConnection (the documented default). A name
// declared more than once (once per conditional branch) is pipeline scoped
// only if every declaration is.
func scopeByField(schema filament.ConfigSchema) map[string]filament.FieldScope {
	out := make(map[string]filament.FieldScope, len(schema.Fields))
	for _, f := range schema.Fields {
		scope := f.Scope
		if scope == filament.ScopeUnspecified {
			scope = filament.ScopeConnection
		}
		if prev, ok := out[f.Name]; ok && prev == filament.ScopeConnection {
			continue
		}
		out[f.Name] = scope
	}
	return out
}

// ValidateConnectionConfig rejects a connection whose config carries a
// PIPELINE-scoped field — those belong on the pipeline node, not the reusable
// connection. Unknown keys (not in the schema) are left to schema validation.
func ValidateConnectionConfig(schema filament.ConfigSchema, cfg map[string]any) error {
	scopes := scopeByField(schema)
	for key := range cfg {
		if scopes[key] == filament.ScopePipeline {
			return fmt.Errorf("field %q is pipeline-scoped and must be set on the pipeline node, not the connection", key)
		}
	}
	return nil
}

// ValidateOverlayConfig rejects a pipeline-node overlay that sets a
// CONNECTION-scoped field — those are owned by the connection and must not be
// overridden per pipeline. Unknown keys pass through untouched.
func ValidateOverlayConfig(schema filament.ConfigSchema, cfg map[string]any) error {
	scopes := scopeByField(schema)
	for key := range cfg {
		if scope, ok := scopes[key]; ok && scope == filament.ScopeConnection {
			return fmt.Errorf("field %q is connection-scoped and cannot be overridden on the pipeline node", key)
		}
	}
	return nil
}

// ValidateOverlaySecretRefs rejects pipeline-node secret refs a node may not
// set. A node may only fill pipeline-scoped secret fields: a ref for a
// connection field would swap out the connection's own secret, just as a
// connection-scoped config key would, and a ref for a non-secret or unknown
// field would write the secret's value where it does not belong. Refs in the
// connection-managed namespace are minted for one connection's field, so a
// node never owns one.
func ValidateOverlaySecretRefs(schema filament.ConfigSchema, refs map[string]string) error {
	scopes := scopeByField(schema)
	for field, ref := range refs {
		top, _, _ := strings.Cut(field, ".")
		if scopes[top] != filament.ScopePipeline {
			return fmt.Errorf("secret ref for field %q: only pipeline-scoped fields can take a secret ref on the pipeline node", field)
		}
		if !isSecretFieldPath(schema.Fields, field) {
			return fmt.Errorf("secret ref for field %q: the field is not a secret field", field)
		}
		if strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
			return fmt.Errorf("secret ref for field %q belongs to a connection and cannot be used on a pipeline node", field)
		}
	}
	return nil
}

// isSecretFieldPath reports whether the dotted path names a secret field in
// fields, descending through object fields. Any declaration of a repeated name
// may match.
func isSecretFieldPath(fields []filament.ConfigField, path string) bool {
	name, rest, nested := strings.Cut(path, ".")
	for _, field := range fields {
		if field.Name != name {
			continue
		}
		if !nested {
			if field.Type == filament.FieldSecret || field.Secret {
				return true
			}
			continue
		}
		if isSecretFieldPath(field.Fields, rest) {
			return true
		}
	}
	return false
}
