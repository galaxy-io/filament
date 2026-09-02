package remote

import (
	"context"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Catalog returns the connector catalog the deployment was built with, so
// dynamic flags and wizards reflect the server's connectors rather than the
// CLI's own build.
func (t *Target) Catalog(ctx context.Context) (model.Catalog, error) {
	catalog := model.Catalog{
		Sources: map[string]filament.ConnectorSpec{},
		Sinks:   map[string]filament.SinkSpec{},
	}
	cursor := ""
	for {
		response, err := t.client.ListConnectors(ctx, connect.NewRequest(&ingestionv1.ListConnectorsRequest{
			Pagination: paginationRequest(internalPageSize, cursor),
		}))
		if err != nil {
			return model.Catalog{}, t.rpcError(err)
		}
		for _, spec := range response.Msg.GetConnectors() {
			switch spec.GetKind() {
			case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
				catalog.Sources[spec.GetName()] = sourceSpecFromProto(spec)
			case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
				catalog.Sinks[spec.GetName()] = sinkSpecFromProto(spec)
			case ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
			}
		}
		cursor = response.Msg.GetPagination().GetNextCursor()
		if cursor == "" {
			return catalog, nil
		}
	}
}

func sourceSpecFromProto(spec *ingestionv1.ConnectorSpec) filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:        spec.GetName(),
		DisplayName: spec.GetDisplayName(),
		Description: spec.GetDescription(),
		Version:     spec.GetVersion(),
		Maturity:    maturityFromProto(spec.GetMaturity()),
		Modes:       readModesFromProto(spec.GetModes()),
		Config:      schemaFromProto(spec.GetConfigSchema()),
	}
}

func sinkSpecFromProto(spec *ingestionv1.ConnectorSpec) filament.SinkSpec {
	return filament.SinkSpec{
		Name:        spec.GetName(),
		DisplayName: spec.GetDisplayName(),
		Description: spec.GetDescription(),
		Version:     spec.GetVersion(),
		Maturity:    maturityFromProto(spec.GetMaturity()),
		Config:      schemaFromProto(spec.GetConfigSchema()),
		SchemaField: spec.GetSchemaField(),
	}
}

func maturityFromProto(maturity ingestionv1.ConnectorMaturity) filament.ConnectorMaturity {
	switch maturity {
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA:
		return filament.MaturityBeta
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_STABLE:
		return filament.MaturityStable
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_ALPHA,
		ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_UNSPECIFIED:
		return filament.MaturityAlpha
	default:
		return filament.MaturityAlpha
	}
}

// readModesFromProto expands replication modes into the read levers they
// allow: standard connections read full or incremental, CDC reads the stream.
func readModesFromProto(modes []ingestionv1.ReplicationMode) []filament.ReadMode {
	var out []filament.ReadMode
	for _, mode := range modes {
		switch mode {
		case ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD:
			out = append(out, filament.ModeFull, filament.ModeIncremental)
		case ingestionv1.ReplicationMode_REPLICATION_MODE_CDC:
			out = append(out, filament.ModeCDC)
		case ingestionv1.ReplicationMode_REPLICATION_MODE_UNSPECIFIED:
		}
	}
	return out
}

func schemaFromProto(schema *ingestionv1.ConfigSchema) filament.ConfigSchema {
	return filament.ConfigSchema{Fields: fieldsFromProto(schema.GetFields())}
}

func fieldsFromProto(fields []*ingestionv1.ConfigField) []filament.ConfigField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]filament.ConfigField, 0, len(fields))
	for _, field := range fields {
		out = append(out, filament.ConfigField{
			Name:        field.GetName(),
			Type:        fieldTypeFromProto(field.GetType()),
			Required:    field.GetRequired(),
			Default:     field.GetDefault().AsInterface(),
			Enum:        enumOptionsFromProto(field.GetEnum()),
			Help:        field.GetHelp(),
			Scope:       fieldScopeFromProto(field.GetScope()),
			Secret:      field.GetSecret(),
			VisibleWhen: fieldConditionFromProto(field.GetVisibleWhen()),
			Fields:      fieldsFromProto(field.GetFields()),
		})
	}
	return out
}

func fieldTypeFromProto(t ingestionv1.FieldType) filament.FieldType {
	switch t {
	case ingestionv1.FieldType_FIELD_TYPE_INT:
		return filament.FieldInt
	case ingestionv1.FieldType_FIELD_TYPE_BOOL:
		return filament.FieldBool
	case ingestionv1.FieldType_FIELD_TYPE_SECRET:
		return filament.FieldSecret
	case ingestionv1.FieldType_FIELD_TYPE_DURATION:
		return filament.FieldDuration
	case ingestionv1.FieldType_FIELD_TYPE_ENUM:
		return filament.FieldEnum
	case ingestionv1.FieldType_FIELD_TYPE_OBJECT:
		return filament.FieldObject
	case ingestionv1.FieldType_FIELD_TYPE_LIST:
		return filament.FieldList
	case ingestionv1.FieldType_FIELD_TYPE_STRING, ingestionv1.FieldType_FIELD_TYPE_UNSPECIFIED:
		return filament.FieldString
	default:
		return filament.FieldString
	}
}

func fieldScopeFromProto(scope ingestionv1.FieldScope) filament.FieldScope {
	switch scope {
	case ingestionv1.FieldScope_FIELD_SCOPE_CONNECTION:
		return filament.ScopeConnection
	case ingestionv1.FieldScope_FIELD_SCOPE_PIPELINE:
		return filament.ScopePipeline
	case ingestionv1.FieldScope_FIELD_SCOPE_UNSPECIFIED:
		return filament.ScopeUnspecified
	default:
		return filament.ScopeUnspecified
	}
}

func enumOptionsFromProto(options []*ingestionv1.EnumOption) []filament.EnumOption {
	if len(options) == 0 {
		return nil
	}
	out := make([]filament.EnumOption, 0, len(options))
	for _, option := range options {
		out = append(out, filament.EnumOption{Value: option.GetValue(), Label: option.GetLabel()})
	}
	return out
}

func fieldConditionFromProto(condition *ingestionv1.FieldCondition) *filament.FieldCondition {
	if condition == nil {
		return nil
	}
	return &filament.FieldCondition{Field: condition.GetField(), Values: condition.GetValues()}
}
