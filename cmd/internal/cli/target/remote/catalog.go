package remote

import (
	"context"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Catalog returns the connectors the deployment was built with, so flags
// and wizards reflect the server rather than the CLI's own build.
func (t *Target) Catalog(ctx context.Context) (model.Catalog, error) {
	specs, err := drain(func(cursor string) ([]*ingestionv1.ConnectorSpec, *ingestionv1.PaginationResponse, error) {
		response, err := t.client.ListConnectors(ctx, connect.NewRequest(&ingestionv1.ListConnectorsRequest{Pagination: pagination(cursor)}))
		if err != nil {
			return nil, nil, t.rpcError(err)
		}
		return response.Msg.GetConnectors(), response.Msg.GetPagination(), nil
	})
	if err != nil {
		return model.Catalog{}, err
	}
	catalog := model.Catalog{
		Sources: map[string]filament.ConnectorSpec{},
		Sinks:   map[string]filament.SinkSpec{},
	}
	for _, spec := range specs {
		switch spec.GetKind() {
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
			catalog.Sources[spec.GetName()] = filament.ConnectorSpec{
				Name:        spec.GetName(),
				DisplayName: spec.GetDisplayName(),
				Description: spec.GetDescription(),
				Version:     spec.GetVersion(),
				Maturity:    maturityFromProto(spec.GetMaturity()),
				Modes:       readModesFromProto(spec.GetModes()),
				Config:      filament.ConfigSchema{Fields: fieldsFromProto(spec.GetConfigSchema().GetFields())},
			}
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
			catalog.Sinks[spec.GetName()] = filament.SinkSpec{
				Name:        spec.GetName(),
				DisplayName: spec.GetDisplayName(),
				Description: spec.GetDescription(),
				Version:     spec.GetVersion(),
				Maturity:    maturityFromProto(spec.GetMaturity()),
				Config:      filament.ConfigSchema{Fields: fieldsFromProto(spec.GetConfigSchema().GetFields())},
				SchemaField: spec.GetSchemaField(),
			}
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		}
	}
	return catalog, nil
}

func maturityFromProto(maturity ingestionv1.ConnectorMaturity) filament.ConnectorMaturity {
	switch maturity {
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA:
		return filament.MaturityBeta
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_STABLE:
		return filament.MaturityStable
	default:
		return filament.MaturityAlpha
	}
}

// readModesFromProto expands replication modes into read levers: standard
// connections read full or incremental, CDC reads the stream.
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
