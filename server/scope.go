package server

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// schemaFor resolves a connector's config schema by kind + name from the source
// and sink registries.
func (a *Server) schemaFor(kind ingestionv1.ConnectorKind, connector string) (filament.ConfigSchema, error) {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		src, err := a.sources.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return src.Spec().Config, nil
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sink, err := a.sinks.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return sink.Spec().Config, nil
	default:
		return filament.ConfigSchema{}, fmt.Errorf("connector kind is required")
	}
}

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

// validateConnectionConfig rejects a connection whose config carries a
// PIPELINE-scoped field — those belong on the pipeline node, not the reusable
// connection. Unknown keys (not in the schema) are left to schema validation.
func validateConnectionConfig(schema filament.ConfigSchema, cfg map[string]any) error {
	scopes := scopeByField(schema)
	for key := range cfg {
		if scopes[key] == filament.ScopePipeline {
			return fmt.Errorf("field %q is pipeline-scoped and must be set on the pipeline node, not the connection", key)
		}
	}
	return nil
}

// validateOverlayConfig rejects a pipeline-node overlay that sets a
// CONNECTION-scoped field — those are owned by the connection and must not be
// overridden per pipeline. Unknown keys pass through untouched.
func validateOverlayConfig(schema filament.ConfigSchema, cfg map[string]any) error {
	scopes := scopeByField(schema)
	for key := range cfg {
		if scope, ok := scopes[key]; ok && scope == filament.ScopeConnection {
			return fmt.Errorf("field %q is connection-scoped and cannot be overridden on the pipeline node", key)
		}
	}
	return nil
}
