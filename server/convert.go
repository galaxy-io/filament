package server

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/events"
)

func sourceSpecToProto(spec filament.ConnectorSpec) *ingestionv1.ConnectorSpec {
	return &ingestionv1.ConnectorSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Description:  spec.Description,
		DarkLogoUrl:  spec.DarkLogoURL,
		LightLogoUrl: spec.LightLogoURL,
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Version:      spec.Version,
		Maturity:     connectorMaturityToProto(spec.Maturity),
		Modes:        modesToProto(spec.Modes),
		ConfigSchema: configSchemaToProto(spec.Config),
	}
}

func connectorMaturityToProto(maturity filament.ConnectorMaturity) ingestionv1.ConnectorMaturity {
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

func sinkSpecToProto(spec filament.SinkSpec) *ingestionv1.ConnectorSpec {
	return &ingestionv1.ConnectorSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Description:  spec.Description,
		DarkLogoUrl:  spec.DarkLogoURL,
		LightLogoUrl: spec.LightLogoURL,
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
		Version:      spec.Version,
		Maturity:     connectorMaturityToProto(spec.Maturity),
		ConfigSchema: configSchemaToProto(spec.Config),
		SchemaField:  spec.SchemaField,
	}
}

func configSchemaToProto(schema filament.ConfigSchema) *ingestionv1.ConfigSchema {
	return &ingestionv1.ConfigSchema{Fields: configFieldsToProto(schema.Fields)}
}

func configFieldsToProto(configFields []filament.ConfigField) []*ingestionv1.ConfigField {
	fields := make([]*ingestionv1.ConfigField, 0, len(configFields))
	for _, field := range configFields {
		fields = append(fields, &ingestionv1.ConfigField{
			Name:        field.Name,
			Type:        fieldTypeToProto(field.Type),
			Required:    field.Required,
			Default:     valueToProto(field.Default),
			Enum:        enumOptionsToProto(field.Enum),
			Help:        field.Help,
			Scope:       fieldScopeToProto(field.Scope),
			Secret:      field.Secret || field.Type == filament.FieldSecret,
			Fields:      configFieldsToProto(field.Fields),
			VisibleWhen: fieldConditionToProto(field.VisibleWhen),
		})
	}
	return fields
}

func enumOptionsToProto(options []filament.EnumOption) []*ingestionv1.EnumOption {
	out := make([]*ingestionv1.EnumOption, 0, len(options))
	for _, option := range options {
		out = append(out, &ingestionv1.EnumOption{
			Value: option.Value,
			Label: option.Label,
		})
	}
	return out
}

func fieldConditionToProto(condition *filament.FieldCondition) *ingestionv1.FieldCondition {
	if condition == nil {
		return nil
	}
	return &ingestionv1.FieldCondition{
		Field:  condition.Field,
		Values: condition.Values,
	}
}

// modesToProto reduces the engine's read mechanisms to the connection-level
// replication modes a connector supports.
func modesToProto(modes []filament.ReadMode) []ingestionv1.ReplicationMode {
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

func replicationToProto(mode filament.ReplicationMode) ingestionv1.ReplicationMode {
	if mode == filament.ReplicationCDC {
		return ingestionv1.ReplicationMode_REPLICATION_MODE_CDC
	}
	return ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD
}

func standardSyncModeFromProto(mode ingestionv1.StandardSyncMode) (filament.StandardSyncMode, error) {
	switch mode {
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_UNSPECIFIED,
		ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE:
		return filament.StandardSyncReplace, nil
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_APPEND:
		return filament.StandardSyncAppend, nil
	case ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL:
		return filament.StandardSyncIncremental, nil
	default:
		return "", fmt.Errorf("unknown Standard sync mode %d", mode)
	}
}

func standardSyncModeToProto(mode filament.StandardSyncMode) ingestionv1.StandardSyncMode {
	switch mode {
	case filament.StandardSyncAppend:
		return ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_APPEND
	case filament.StandardSyncIncremental:
		return ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL
	default:
		return ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE
	}
}

func fieldTypeToProto(t filament.FieldType) ingestionv1.FieldType {
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

func fieldScopeToProto(s filament.FieldScope) ingestionv1.FieldScope {
	switch s {
	case filament.ScopeConnection:
		return ingestionv1.FieldScope_FIELD_SCOPE_CONNECTION
	case filament.ScopePipeline:
		return ingestionv1.FieldScope_FIELD_SCOPE_PIPELINE
	default:
		return ingestionv1.FieldScope_FIELD_SCOPE_UNSPECIFIED
	}
}

func runStatusToProto(status filament.RunStatus) ingestionv1.RunStatus {
	switch status {
	case filament.RunRequested:
		return ingestionv1.RunStatus_RUN_STATUS_REQUESTED
	case filament.RunRunning:
		return ingestionv1.RunStatus_RUN_STATUS_RUNNING
	case filament.RunCompleted:
		return ingestionv1.RunStatus_RUN_STATUS_COMPLETED
	case filament.RunFailed:
		return ingestionv1.RunStatus_RUN_STATUS_FAILED
	case filament.RunCanceled:
		return ingestionv1.RunStatus_RUN_STATUS_CANCELED
	case filament.RunPaused:
		return ingestionv1.RunStatus_RUN_STATUS_PAUSED
	case filament.RunPartial:
		return ingestionv1.RunStatus_RUN_STATUS_PARTIAL
	case filament.RunScheduled:
		return ingestionv1.RunStatus_RUN_STATUS_SCHEDULED
	default:
		return ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func runStatusesFromProto(statuses []ingestionv1.RunStatus) []filament.RunStatus {
	out := make([]filament.RunStatus, 0, len(statuses))
	for _, status := range statuses {
		switch status {
		case ingestionv1.RunStatus_RUN_STATUS_REQUESTED:
			out = append(out, filament.RunRequested)
		case ingestionv1.RunStatus_RUN_STATUS_RUNNING:
			out = append(out, filament.RunRunning)
		case ingestionv1.RunStatus_RUN_STATUS_COMPLETED:
			out = append(out, filament.RunCompleted)
		case ingestionv1.RunStatus_RUN_STATUS_FAILED:
			out = append(out, filament.RunFailed)
		case ingestionv1.RunStatus_RUN_STATUS_CANCELED:
			out = append(out, filament.RunCanceled)
		case ingestionv1.RunStatus_RUN_STATUS_PAUSED:
			out = append(out, filament.RunPaused)
		case ingestionv1.RunStatus_RUN_STATUS_PARTIAL:
			out = append(out, filament.RunPartial)
		case ingestionv1.RunStatus_RUN_STATUS_SCHEDULED:
			out = append(out, filament.RunScheduled)
		}
	}
	return out
}

func resourcesToProto(resources []filament.Resource) *ingestionv1.DiscoverResourcesResponse {
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
	return &ingestionv1.DiscoverResourcesResponse{Resources: out}
}

// epochMillis renders a stamp for the wire, where 0 means unset.
func epochMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func runInfoToProto(state filament.RunState) *ingestionv1.RunInfo {
	var endedAt int64
	if state.EndedAt != nil {
		endedAt = state.EndedAt.UnixMilli()
	}
	return &ingestionv1.RunInfo{
		Id:                 string(state.Run),
		TenantId:           string(state.Tenant),
		PipelineId:         state.Request.PipelineID,
		PipelineVersionId:  state.Request.PipelineVersionID,
		Status:             runStatusToProto(state.Status),
		Records:            state.Records,
		Bytes:              state.Bytes,
		Error:              state.Error,
		StartedAt:          epochMillis(state.StartedAt),
		EndedAt:            endedAt,
		SourceConnectionId: state.Request.SourceConnectionID,
		SinkConnectionId:   state.Request.SinkConnectionID,
		CpuSeconds:         state.CPUSeconds,
		MemoryPeakBytes:    state.MemoryPeakBytes,
		CreatedAt:          epochMillis(state.CreatedAt),
		ScheduledAt:        epochMillis(state.ScheduledAt),
		RequestedAt:        epochMillis(state.RequestedAt),
		UpdatedAt:          epochMillis(state.UpdatedAt),
	}
}

func eventToProto(f events.Fact, replay bool) *ingestionv1.RunEvent {
	return &ingestionv1.RunEvent{
		EventType: f.Name,
		TenantId:  string(f.Tenant),
		RunId:     string(f.Run),
		Resource:  f.Resource,
		Seq:       f.Seq,
		CreatedAt: f.At.UnixMilli(),
		Fields:    eventFieldsToProto(f.Data),
		IsReplay:  replay,
	}
}

// eventFieldsToProto flattens a typed payload into the proto field union.
func eventFieldsToProto(data any) *ingestionv1.RunEventFields {
	fields := &ingestionv1.RunEventFields{}
	switch d := data.(type) {
	case events.RunCompletedEvent:
		fields.Records, fields.Bytes = d.Records, d.Bytes
	case events.RunFailedEvent:
		fields.Error = d.Error
	case events.RunPartialEvent:
		fields.Error = d.Error
	case events.PageFetchedEvent:
		fields.Records, fields.Bytes, fields.Uri = d.Records, d.Bytes, d.URI
	case events.ResourceCompletedEvent:
		fields.Records, fields.Bytes = d.Records, d.Bytes
	case events.ResourceFailedEvent:
		fields.Error = d.Error
	case events.BatchBufferedEvent:
		fields.Records, fields.Bytes = d.Records, d.Bytes
	case events.BatchWrittenEvent:
		fields.Records, fields.Bytes, fields.Uri, fields.Crc = d.Records, d.Bytes, d.URI, d.CRC
	case events.IntegrityVerifiedEvent:
		fields.Crc = d.CRC
	case events.ChunkDivergenceEvent:
		fields.Crc, fields.Error = d.CRC, d.Error
	case events.RetryExhaustedEvent:
		fields.Error = d.Error
	}
	return fields
}

func tailResponse(ev *ingestionv1.RunEvent) *ingestionv1.TailRunResponse {
	return &ingestionv1.TailRunResponse{Event: ev}
}

func runSnapshotEvent(state filament.RunState, replay bool) *ingestionv1.RunEvent {
	return &ingestionv1.RunEvent{
		EventType: runEventType(state.Status),
		TenantId:  string(state.Tenant),
		RunId:     string(state.Run),
		Fields: &ingestionv1.RunEventFields{
			Records: state.Records,
			Bytes:   state.Bytes,
			Error:   state.Error,
		},
		IsReplay: replay,
	}
}

func runStatusTerminal(status filament.RunStatus) bool {
	switch status {
	case filament.RunCompleted, filament.RunFailed, filament.RunCanceled, filament.RunPartial:
		return true
	default:
		return false
	}
}

func runEventType(status filament.RunStatus) string {
	switch status {
	case filament.RunRequested:
		return events.RunRequested.Name()
	case filament.RunRunning:
		return events.RunStarted.Name()
	case filament.RunCompleted:
		return events.RunCompleted.Name()
	case filament.RunFailed:
		return events.RunFailed.Name()
	case filament.RunCanceled:
		return "run.canceled"
	case filament.RunPaused:
		return "run.paused"
	case filament.RunPartial:
		return events.RunPartial.Name()
	default:
		return "unspecified"
	}
}

func runStatusEventType(status filament.RunStatus) string {
	switch status {
	case filament.RunRequested:
		return events.RunRequested.Name()
	case filament.RunRunning:
		return events.ResourceStarted.Name()
	case filament.RunCompleted:
		return events.ResourceCompleted.Name()
	case filament.RunFailed:
		return events.ResourceFailed.Name()
	case filament.RunCanceled:
		return "run.canceled"
	case filament.RunPaused:
		return "run.paused"
	case filament.RunPartial:
		return events.RunPartial.Name()
	default:
		return "unspecified"
	}
}

func structMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return s.AsMap()
}

func valueToProto(v any) *structpb.Value {
	if v == nil {
		return nil
	}
	value, err := structpb.NewValue(v)
	if err != nil {
		return nil
	}
	return value
}

func validationError(message string) *ingestionv1.ValidateConfigResponse {
	return &ingestionv1.ValidateConfigResponse{
		Valid: false,
		Errors: []*ingestionv1.ValidationError{{
			Message: message,
		}},
	}
}

func validateConfigSchema(schema filament.ConfigSchema, cfg filament.Config, scope filament.FieldScope) error {
	for _, field := range schema.Fields {
		if field.Scope != scope {
			continue
		}
		if err := validateConfigField(field, cfg, field.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigField(field filament.ConfigField, cfg filament.Config, path string) error {
	if !fieldIsVisible(field, cfg) {
		return nil
	}
	if field.Required && !cfg.Has(field.Name) {
		return fmt.Errorf("%s is required", path)
	}
	if field.Required && field.Type == filament.FieldList && configListLen(cfg.Raw()[field.Name]) == 0 {
		return fmt.Errorf("%s is required", path)
	}
	if !cfg.Has(field.Name) || len(field.Fields) == 0 {
		return nil
	}
	nested := cfg.Sub(field.Name)
	for _, child := range field.Fields {
		if err := validateConfigField(child, nested, path+"."+child.Name); err != nil {
			return err
		}
	}
	return nil
}

func configListLen(value any) int {
	switch value := value.(type) {
	case []string:
		return len(value)
	case []any:
		return len(value)
	case string:
		count := 0
		for _, item := range strings.Split(value, ",") {
			if strings.TrimSpace(item) != "" {
				count++
			}
		}
		return count
	default:
		return 0
	}
}

func fieldIsVisible(field filament.ConfigField, cfg filament.Config) bool {
	if field.VisibleWhen == nil {
		return true
	}
	actual := cfg.String(field.VisibleWhen.Field)
	for _, value := range field.VisibleWhen.Values {
		if actual == value {
			return true
		}
	}
	return false
}

// runOptionsFromProto maps the proto override message onto filament.RunOptions.
// A nil message or any zero field defers to the engine default downstream.
func runOptionsFromProto(o *ingestionv1.RunOptions) filament.RunOptions {
	if o == nil {
		return filament.RunOptions{}
	}
	opts := filament.RunOptions{
		FetchSize:           int(o.GetFetchSize()),
		BatchMaxRows:        int(o.GetBatchMaxRows()),
		BatchMaxBytes:       o.GetBatchMaxBytes(),
		SnapshotParallelism: int(o.GetSnapshotParallelism()),
		CheckpointEvery:     int(o.GetCheckpointEvery()),
	}
	if rl := o.GetRateLimit(); rl != nil {
		opts.RateLimit = &filament.RatePolicy{
			RequestsPerSecond: rl.GetRequestsPerSecond(),
			Burst:             int(rl.GetBurst()),
		}
	}
	return opts
}

func defaultTenant(tenant string) string {
	if tenant == "" {
		return string(filament.DefaultTenantID)
	}
	return tenant
}
