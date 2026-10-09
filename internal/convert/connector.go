// Package convert maps root types onto the generated API types and back. It is
// the only package that knows both shapes: the public ingestion.v1 handlers,
// the worker handler and the remote worker client all encode and decode here,
// so every type has exactly one wire form.
package convert

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// SourceSpecToProto is the public projection of a source spec. Read modes
// collapse to replication modes and policies, resources and stream
// capabilities are omitted; the worker contract carries those.
func SourceSpecToProto(spec filament.ConnectorSpec) *ingestionv1.ConnectorSpec {
	return &ingestionv1.ConnectorSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Description:  spec.Description,
		DarkLogoUrl:  spec.DarkLogoURL,
		LightLogoUrl: spec.LightLogoURL,
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Version:      spec.Version,
		ApiVersion:   spec.APIVersion,
		AliasTarget:  spec.AliasTarget,
		Maturity:     ConnectorMaturityToProto(spec.Maturity),
		Modes:        ReplicationModesToProto(spec.Modes),
		ConfigSchema: ConfigSchemaToProto(spec.Config),
	}
}

// sourceSpecFromProto restores the fields the public projection carries.
func sourceSpecFromProto(spec *ingestionv1.ConnectorSpec) (filament.ConnectorSpec, error) {
	config, err := ConfigSchemaFromProto(spec.GetConfigSchema())
	if err != nil {
		return filament.ConnectorSpec{}, err
	}
	maturity, err := ConnectorMaturityFromProto(spec.GetMaturity())
	if err != nil {
		return filament.ConnectorSpec{}, err
	}
	return filament.ConnectorSpec{
		Name:         spec.GetName(),
		DisplayName:  spec.GetDisplayName(),
		Description:  spec.GetDescription(),
		DarkLogoURL:  spec.GetDarkLogoUrl(),
		LightLogoURL: spec.GetLightLogoUrl(),
		Version:      spec.GetVersion(),
		APIVersion:   spec.GetApiVersion(),
		AliasTarget:  spec.GetAliasTarget(),
		Maturity:     maturity,
		Config:       config,
	}, nil
}

// SinkSpecToProto is the public projection of a sink spec; capabilities are
// omitted and travel on the worker contract.
func SinkSpecToProto(spec filament.SinkSpec) *ingestionv1.ConnectorSpec {
	return &ingestionv1.ConnectorSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Description:  spec.Description,
		DarkLogoUrl:  spec.DarkLogoURL,
		LightLogoUrl: spec.LightLogoURL,
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
		Version:      spec.Version,
		Maturity:     ConnectorMaturityToProto(spec.Maturity),
		ConfigSchema: ConfigSchemaToProto(spec.Config),
		SchemaField:  spec.SchemaField,
	}
}

func sinkSpecFromProto(spec *ingestionv1.ConnectorSpec) (filament.SinkSpec, error) {
	config, err := ConfigSchemaFromProto(spec.GetConfigSchema())
	if err != nil {
		return filament.SinkSpec{}, err
	}
	maturity, err := ConnectorMaturityFromProto(spec.GetMaturity())
	if err != nil {
		return filament.SinkSpec{}, err
	}
	return filament.SinkSpec{
		Name:         spec.GetName(),
		DisplayName:  spec.GetDisplayName(),
		Description:  spec.GetDescription(),
		DarkLogoURL:  spec.GetDarkLogoUrl(),
		LightLogoURL: spec.GetLightLogoUrl(),
		Version:      spec.GetVersion(),
		Maturity:     maturity,
		Config:       config,
		SchemaField:  spec.GetSchemaField(),
	}, nil
}

// ConnectorMaturityToProto maps a maturity marker; unknown is unspecified.
func ConnectorMaturityToProto(maturity filament.ConnectorMaturity) ingestionv1.ConnectorMaturity {
	switch maturity {
	case filament.MaturityAlpha:
		return ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_ALPHA
	case filament.MaturityBeta:
		return ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA
	case filament.MaturityStable:
		return ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_STABLE
	default:
		return ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_UNSPECIFIED
	}
}

// ConnectorMaturityFromProto accepts unspecified as the empty marker, since a
// spec may leave maturity to its registration.
func ConnectorMaturityFromProto(maturity ingestionv1.ConnectorMaturity) (filament.ConnectorMaturity, error) {
	switch maturity {
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_UNSPECIFIED:
		return "", nil
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_ALPHA:
		return filament.MaturityAlpha, nil
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA:
		return filament.MaturityBeta, nil
	case ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_STABLE:
		return filament.MaturityStable, nil
	default:
		return "", fmt.Errorf("unknown connector maturity %d", maturity)
	}
}

// ConfigSchemaToProto encodes a config schema. A field typed secret is marked
// secret whether or not the spec said so.
func ConfigSchemaToProto(schema filament.ConfigSchema) *ingestionv1.ConfigSchema {
	return &ingestionv1.ConfigSchema{Fields: configFieldsToProto(schema.Fields)}
}

// ConfigSchemaFromProto decodes a config schema. Defaults come back as the
// JSON number and object types a Struct value carries.
func ConfigSchemaFromProto(schema *ingestionv1.ConfigSchema) (filament.ConfigSchema, error) {
	fields, err := configFieldsFromProto(schema.GetFields())
	if err != nil {
		return filament.ConfigSchema{}, err
	}
	return filament.ConfigSchema{Fields: fields}, nil
}

func configFieldsToProto(configFields []filament.ConfigField) []*ingestionv1.ConfigField {
	fields := make([]*ingestionv1.ConfigField, 0, len(configFields))
	for _, field := range configFields {
		fields = append(fields, &ingestionv1.ConfigField{
			Name:        field.Name,
			Type:        FieldTypeToProto(field.Type),
			Required:    field.Required,
			Default:     ValueToProto(field.Default),
			Enum:        enumOptionsToProto(field.Enum),
			Help:        field.Help,
			Scope:       FieldScopeToProto(field.Scope),
			Secret:      field.Secret || field.Type == filament.FieldSecret,
			Fields:      configFieldsToProto(field.Fields),
			VisibleWhen: fieldConditionToProto(field.VisibleWhen),
		})
	}
	return fields
}

func configFieldsFromProto(configFields []*ingestionv1.ConfigField) ([]filament.ConfigField, error) {
	if len(configFields) == 0 {
		return nil, nil
	}
	fields := make([]filament.ConfigField, 0, len(configFields))
	for _, field := range configFields {
		fieldType, err := FieldTypeFromProto(field.GetType())
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field.GetName(), err)
		}
		scope, err := FieldScopeFromProto(field.GetScope())
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field.GetName(), err)
		}
		nested, err := configFieldsFromProto(field.GetFields())
		if err != nil {
			return nil, err
		}
		var defaultValue any
		if field.GetDefault() != nil {
			defaultValue = field.GetDefault().AsInterface()
		}
		fields = append(fields, filament.ConfigField{
			Name:        field.GetName(),
			Type:        fieldType,
			Required:    field.GetRequired(),
			Default:     defaultValue,
			Enum:        enumOptionsFromProto(field.GetEnum()),
			Help:        field.GetHelp(),
			Scope:       scope,
			Secret:      field.GetSecret(),
			Fields:      nested,
			VisibleWhen: fieldConditionFromProto(field.GetVisibleWhen()),
		})
	}
	return fields, nil
}

func enumOptionsToProto(options []filament.EnumOption) []*ingestionv1.EnumOption {
	out := make([]*ingestionv1.EnumOption, 0, len(options))
	for _, option := range options {
		out = append(out, &ingestionv1.EnumOption{Value: option.Value, Label: option.Label})
	}
	return out
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

func fieldConditionToProto(condition *filament.FieldCondition) *ingestionv1.FieldCondition {
	if condition == nil {
		return nil
	}
	return &ingestionv1.FieldCondition{Field: condition.Field, Values: condition.Values}
}

func fieldConditionFromProto(condition *ingestionv1.FieldCondition) *filament.FieldCondition {
	if condition == nil {
		return nil
	}
	return &filament.FieldCondition{Field: condition.GetField(), Values: condition.GetValues()}
}

// ReplicationModesToProto reduces the engine's read mechanisms to the
// connection-level replication modes a connector supports.
func ReplicationModesToProto(modes []filament.ReadMode) []ingestionv1.ReplicationMode {
	var standard, cdc bool
	for _, mode := range modes {
		if mode == filament.ModeCDC {
			cdc = true
		} else {
			standard = true
		}
	}
	var out []ingestionv1.ReplicationMode
	if standard {
		out = append(out, ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD)
	}
	if cdc {
		out = append(out, ingestionv1.ReplicationMode_REPLICATION_MODE_CDC)
	}
	return out
}

// ReplicationToProto maps a connection's replication mode.
func ReplicationToProto(mode filament.ReplicationMode) ingestionv1.ReplicationMode {
	if mode == filament.ReplicationCDC {
		return ingestionv1.ReplicationMode_REPLICATION_MODE_CDC
	}
	return ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD
}

// ReadModeFromProto maps a read mode; unspecified means full.
func ReadModeFromProto(mode ingestionv1.ReadMode) (filament.ReadMode, error) {
	switch mode {
	case ingestionv1.ReadMode_READ_MODE_UNSPECIFIED, ingestionv1.ReadMode_READ_MODE_FULL:
		return filament.ModeFull, nil
	case ingestionv1.ReadMode_READ_MODE_INCREMENTAL:
		return filament.ModeIncremental, nil
	case ingestionv1.ReadMode_READ_MODE_CDC:
		return filament.ModeCDC, nil
	default:
		return 0, fmt.Errorf("unknown read mode %d", mode)
	}
}

// ReadModeToProto maps a read mode.
func ReadModeToProto(mode filament.ReadMode) ingestionv1.ReadMode {
	switch mode {
	case filament.ModeIncremental:
		return ingestionv1.ReadMode_READ_MODE_INCREMENTAL
	case filament.ModeCDC:
		return ingestionv1.ReadMode_READ_MODE_CDC
	default:
		return ingestionv1.ReadMode_READ_MODE_FULL
	}
}

// WriteModeFromProto maps a write mode; unspecified means upsert.
func WriteModeFromProto(mode ingestionv1.WriteMode) (filament.WriteMode, error) {
	switch mode {
	case ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED:
		return filament.WriteUpsert, nil
	case ingestionv1.WriteMode_WRITE_MODE_REPLACE:
		return filament.WriteReplace, nil
	case ingestionv1.WriteMode_WRITE_MODE_APPEND:
		return filament.WriteAppend, nil
	case ingestionv1.WriteMode_WRITE_MODE_UPSERT:
		return filament.WriteUpsert, nil
	case ingestionv1.WriteMode_WRITE_MODE_MERGE:
		return filament.WriteMerge, nil
	case ingestionv1.WriteMode_WRITE_MODE_DELETE:
		return filament.WriteDelete, nil
	default:
		return "", fmt.Errorf("unknown write mode %d", mode)
	}
}

// WriteModeToProto maps a write mode; unknown is unspecified.
func WriteModeToProto(mode filament.WriteMode) ingestionv1.WriteMode {
	switch mode {
	case filament.WriteAppend:
		return ingestionv1.WriteMode_WRITE_MODE_APPEND
	case filament.WriteUpsert:
		return ingestionv1.WriteMode_WRITE_MODE_UPSERT
	case filament.WriteMerge:
		return ingestionv1.WriteMode_WRITE_MODE_MERGE
	case filament.WriteReplace:
		return ingestionv1.WriteMode_WRITE_MODE_REPLACE
	case filament.WriteDelete:
		return ingestionv1.WriteMode_WRITE_MODE_DELETE
	default:
		return ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED
	}
}

// FieldTypeToProto maps a config field type; unknown is unspecified.
func FieldTypeToProto(t filament.FieldType) ingestionv1.FieldType {
	switch t {
	case filament.FieldString:
		return ingestionv1.FieldType_FIELD_TYPE_STRING
	case filament.FieldInt:
		return ingestionv1.FieldType_FIELD_TYPE_INT
	case filament.FieldBool:
		return ingestionv1.FieldType_FIELD_TYPE_BOOL
	case filament.FieldSecret:
		return ingestionv1.FieldType_FIELD_TYPE_SECRET
	case filament.FieldDuration:
		return ingestionv1.FieldType_FIELD_TYPE_DURATION
	case filament.FieldEnum:
		return ingestionv1.FieldType_FIELD_TYPE_ENUM
	case filament.FieldObject:
		return ingestionv1.FieldType_FIELD_TYPE_OBJECT
	case filament.FieldList:
		return ingestionv1.FieldType_FIELD_TYPE_LIST
	default:
		return ingestionv1.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}

// FieldTypeFromProto maps a config field type; unspecified is an error since
// every field declares one.
func FieldTypeFromProto(t ingestionv1.FieldType) (filament.FieldType, error) {
	switch t {
	case ingestionv1.FieldType_FIELD_TYPE_STRING:
		return filament.FieldString, nil
	case ingestionv1.FieldType_FIELD_TYPE_INT:
		return filament.FieldInt, nil
	case ingestionv1.FieldType_FIELD_TYPE_BOOL:
		return filament.FieldBool, nil
	case ingestionv1.FieldType_FIELD_TYPE_SECRET:
		return filament.FieldSecret, nil
	case ingestionv1.FieldType_FIELD_TYPE_DURATION:
		return filament.FieldDuration, nil
	case ingestionv1.FieldType_FIELD_TYPE_ENUM:
		return filament.FieldEnum, nil
	case ingestionv1.FieldType_FIELD_TYPE_OBJECT:
		return filament.FieldObject, nil
	case ingestionv1.FieldType_FIELD_TYPE_LIST:
		return filament.FieldList, nil
	default:
		return 0, fmt.Errorf("unknown field type %d", t)
	}
}

// FieldScopeToProto maps a config field scope.
func FieldScopeToProto(s filament.FieldScope) ingestionv1.FieldScope {
	switch s {
	case filament.ScopeConnection:
		return ingestionv1.FieldScope_FIELD_SCOPE_CONNECTION
	case filament.ScopePipeline:
		return ingestionv1.FieldScope_FIELD_SCOPE_PIPELINE
	default:
		return ingestionv1.FieldScope_FIELD_SCOPE_UNSPECIFIED
	}
}

// FieldScopeFromProto maps a config field scope; unspecified is a valid scope.
func FieldScopeFromProto(s ingestionv1.FieldScope) (filament.FieldScope, error) {
	switch s {
	case ingestionv1.FieldScope_FIELD_SCOPE_UNSPECIFIED:
		return filament.ScopeUnspecified, nil
	case ingestionv1.FieldScope_FIELD_SCOPE_CONNECTION:
		return filament.ScopeConnection, nil
	case ingestionv1.FieldScope_FIELD_SCOPE_PIPELINE:
		return filament.ScopePipeline, nil
	default:
		return 0, fmt.Errorf("unknown field scope %d", s)
	}
}

// ResourcesToProto is the public projection of discovered resources; row
// estimates and embedded schemas are not carried.
func ResourcesToProto(resources []filament.Resource) []*ingestionv1.Resource {
	out := make([]*ingestionv1.Resource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, &ingestionv1.Resource{
			Name:         resource.Name,
			IsSelectable: resource.Selectable,
			PrimaryKey:   resource.PrimaryKey,
			Selector:     resource.Selector,
			DisplayName:  resource.DisplayName,
			Metadata:     resource.Metadata,
		})
	}
	return out
}

// ResourcesFromProto restores the public projection of discovered resources.
func ResourcesFromProto(resources []*ingestionv1.Resource) []filament.Resource {
	out := make([]filament.Resource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, filament.Resource{
			Name:        resource.GetName(),
			Selectable:  resource.GetIsSelectable(),
			PrimaryKey:  resource.GetPrimaryKey(),
			Selector:    resource.GetSelector(),
			DisplayName: resource.GetDisplayName(),
			Metadata:    resource.GetMetadata(),
		})
	}
	return out
}

// CursorColumnsToProto encodes columns; precision and scale are not carried.
func CursorColumnsToProto(columns []filament.CursorColumn) []*ingestionv1.ResourceColumn {
	out := make([]*ingestionv1.ResourceColumn, 0, len(columns))
	for _, column := range columns {
		out = append(out, &ingestionv1.ResourceColumn{
			Name: column.Name, LogicalType: string(column.Logical), NativeType: column.Native,
			IsNullable: column.Nullable, IsPrimaryKey: column.PrimaryKey,
			IsCursorEligible: column.Eligible, IsCursorRecommended: column.Recommended,
			RecommendationRank: int32(column.Rank), Warning: column.Warning, //nolint:gosec // tiny rank
			IsConfigurable: column.Configurable, SupportsLookback: column.SupportsLookback,
		})
	}
	return out
}

// CursorColumnsFromProto decodes columns.
func CursorColumnsFromProto(columns []*ingestionv1.ResourceColumn) []filament.CursorColumn {
	if len(columns) == 0 {
		return nil
	}
	out := make([]filament.CursorColumn, 0, len(columns))
	for _, column := range columns {
		out = append(out, filament.CursorColumn{
			SchemaField: filament.SchemaField{
				Name:     column.GetName(),
				Logical:  filament.LogicalType(column.GetLogicalType()),
				Native:   column.GetNativeType(),
				Nullable: column.GetIsNullable(),
			},
			PrimaryKey:       column.GetIsPrimaryKey(),
			Eligible:         column.GetIsCursorEligible(),
			Recommended:      column.GetIsCursorRecommended(),
			Rank:             int(column.GetRecommendationRank()),
			Configurable:     column.GetIsConfigurable(),
			SupportsLookback: column.GetSupportsLookback(),
			Warning:          column.GetWarning(),
		})
	}
	return out
}

// StructMap reads a Struct as a map; nil is an empty map.
func StructMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return s.AsMap()
}

// StructFromMap encodes JSON-compatible config values, including typed slices
// and structs with JSON field names. Nil is an empty Struct.
func StructFromMap(m map[string]any) (*structpb.Struct, error) {
	if m == nil {
		return &structpb.Struct{}, nil
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	out := new(structpb.Struct)
	if err := protojson.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ValueToProto encodes one default value; nil and unrepresentable values are
// omitted.
func ValueToProto(v any) *structpb.Value {
	if v == nil {
		return nil
	}
	value, err := structpb.NewValue(v)
	if err != nil {
		return nil
	}
	return value
}

// ConnectorKindToProto maps a connector kind.
func ConnectorKindToProto(k filament.ConnectorKind) ingestionv1.ConnectorKind {
	switch k {
	case filament.ConnectorKindSource:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE
	case filament.ConnectorKindSink:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK
	default:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED
	}
}

// ConnectorKindFromProto maps a connector kind; unknown is unspecified.
func ConnectorKindFromProto(k ingestionv1.ConnectorKind) filament.ConnectorKind {
	switch k {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		return filament.ConnectorKindSource
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		return filament.ConnectorKindSink
	default:
		return filament.ConnectorKindUnspecified
	}
}
