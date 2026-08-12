package compile

import (
	"fmt"

	"github.com/galaxy-io/filament"
)

// scopeByField indexes a schema's fields by name -> scope. Fields left
// unspecified are reported as ScopeConnection (the documented default).
func scopeByField(schema filament.ConfigSchema) map[string]filament.FieldScope {
	out := make(map[string]filament.FieldScope, len(schema.Fields))
	for _, f := range schema.Fields {
		scope := f.Scope
		if scope == filament.ScopeUnspecified {
			scope = filament.ScopeConnection
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
