package server

import (
	"context"
	"reflect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// savedConnectionConfig builds the config a request against a saved
// connection runs with: the saved config with the request's values laid over
// it. The saved secrets are filled in only while every connection field still
// matches the saved connection; once the request changes one, the caller must
// supply the secrets again.
func (a *Server) savedConnectionConfig(ctx context.Context, conn filament.Connection, kind ingestionv1.ConnectorKind, requested map[string]any) (map[string]any, error) {
	schema, err := a.schemaFor(ctx, kind, conn.Connector)
	if err != nil {
		// Without the schema there is no telling which fields are safe to
		// change, so fail closed.
		return nil, connectorError(err)
	}
	base := cloneConfigMap(conn.Config)
	if changedConnectionField(schema, conn.Config, requested) == "" {
		if err := a.resolveConnectionSecrets(ctx, conn, base); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	return overlayConfig(base, requested), nil
}

// changedConnectionField returns the first connection field whose value
// differs between saved and saved-with-requested-laid-over-it, or "" when the
// request leaves every connection field as saved. Pipeline-scoped fields may
// change freely and secret fields are ignored; a field with no scope counts as
// a connection field, as it does for pipeline nodes. Blank strings mean "not
// supplied", absent fields take their schema default, and inactive branches
// are pruned, so an unchanged edit form compares equal.
func changedConnectionField(schema filament.ConfigSchema, saved, requested map[string]any) string {
	before := comparableConnectionConfig(schema, saved)
	after := comparableConnectionConfig(schema, overlayConfig(saved, requested))
	pipeline, defaults := pipelineFieldsAndDefaults(schema)
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	for key := range keys {
		if pipeline[key] {
			continue
		}
		was, ok := before[key]
		if !ok {
			was = normalizeConfigValue(defaults[key])
		}
		now, ok := after[key]
		if !ok {
			now = normalizeConfigValue(defaults[key])
		}
		if !reflect.DeepEqual(was, now) {
			return key
		}
	}
	return ""
}

// comparableConnectionConfig is cfg canonicalized as it would be saved, with
// secret fields removed and values normalized to their JSON types.
func comparableConnectionConfig(schema filament.ConfigSchema, cfg map[string]any) map[string]any {
	out := cloneConfigMap(cfg)
	canonicalizeConnectionConfig(schema, out, nil)
	// Removing secrets is a side effect of extraction; an extraction error
	// only means a malformed secret value, which is ignored here either way.
	_, _ = extractSecretFields(schema.Fields, out, "", true)
	for key, value := range out {
		out[key] = normalizeConfigValue(value)
	}
	return out
}

// pipelineFieldsAndDefaults reports which top-level fields are pipeline
// scoped and each field's default. A name declared once per conditional
// branch is pipeline scoped only if every declaration is.
func pipelineFieldsAndDefaults(schema filament.ConfigSchema) (map[string]bool, map[string]any) {
	pipeline := map[string]bool{}
	defaults := map[string]any{}
	for _, field := range schema.Fields {
		isPipeline := field.Scope.IsPipeline()
		if seen, ok := pipeline[field.Name]; ok {
			isPipeline = isPipeline && seen
		}
		pipeline[field.Name] = isPipeline
		if _, ok := defaults[field.Name]; !ok && field.Default != nil {
			defaults[field.Name] = field.Default
		}
	}
	return pipeline, defaults
}

// normalizeConfigValue converts v to the types a JSON round trip produces, so
// an int default compares equal to the float64 a request carries.
func normalizeConfigValue(v any) any {
	if v == nil {
		return nil
	}
	value, err := structpb.NewValue(v)
	if err != nil {
		return v
	}
	return value.AsInterface()
}
