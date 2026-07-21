package server

import (
	"fmt"

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
		Modes:        modesToProto(spec.Modes),
		ConfigSchema: configSchemaToProto(spec.Config),
		Capabilities: &ingestionv1.Capabilities{
			Discoverable:      spec.Resources.Discoverable,
			PerResourceCursor: spec.Resources.PerResourceCursor,
			SourcePolicies:    sourcePoliciesToProto(spec.SourcePolicies),
		},
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
		ConfigSchema: configSchemaToProto(spec.Config),
		Capabilities: &ingestionv1.Capabilities{
			Transactional: spec.Capabilities.Transactional,
			Upsertable:    spec.Capabilities.Upsertable,
			Schematized:   spec.Capabilities.Schematized,
			WritePolicies: writePolicyCapabilitiesToProto(spec.Capabilities.WritePolicies),
		},
	}
}

func configSchemaToProto(schema filament.ConfigSchema) *ingestionv1.ConfigSchema {
	fields := make([]*ingestionv1.ConfigField, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		fields = append(fields, &ingestionv1.ConfigField{
			Name:     field.Name,
			Type:     fieldTypeToProto(field.Type),
			Required: field.Required,
			Default:  valueToProto(field.Default),
			Enum:     field.Enum,
			Help:     field.Help,
			Scope:    fieldScopeToProto(field.Scope),
			Secret:   field.Secret || field.Type == filament.FieldSecret,
		})
	}
	return &ingestionv1.ConfigSchema{Fields: fields}
}

func sourcePoliciesToProto(policies []filament.SourcePolicy) []*ingestionv1.SourcePolicy {
	out := make([]*ingestionv1.SourcePolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, &ingestionv1.SourcePolicy{
			Mode:          modeToProto(policy.Mode),
			EmitsOps:      operationsToProto(policy.EmitsOps),
			Ordered:       policy.Ordered,
			Checkpointing: checkpointPolicyToProto(policy.Checkpointing),
		})
	}
	return out
}

func writePolicyCapabilitiesToProto(caps []filament.WritePolicyCapability) []*ingestionv1.WritePolicyCapability {
	out := make([]*ingestionv1.WritePolicyCapability, 0, len(caps))
	for _, cap := range caps {
		out = append(out, &ingestionv1.WritePolicyCapability{
			Mode:          writeModeToProto(cap.Mode),
			RequiresPk:    cap.RequiresPK,
			RequiresOrder: cap.RequiresOrder,
			AcceptsOps:    operationsToProto(cap.AcceptsOps),
			Atomicity:     writeAtomicityToProto(cap.Atomicity),
		})
	}
	return out
}

func modesToProto(modes []filament.ReplicationMode) []ingestionv1.ReplicationMode {
	out := make([]ingestionv1.ReplicationMode, 0, len(modes))
	for _, mode := range modes {
		out = append(out, modeToProto(mode))
	}
	return out
}

func modeToProto(mode filament.ReplicationMode) ingestionv1.ReplicationMode {
	switch mode {
	case filament.ModeFull:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_FULL
	case filament.ModeIncremental:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_INCREMENTAL
	case filament.ModeCDC:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_CDC
	default:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_UNSPECIFIED
	}
}

func ingestionTypeFromProto(t ingestionv1.IngestionType) filament.IngestionType {
	switch t {
	case ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT:
		return filament.IngestionSnapshotUpsert
	case ingestionv1.IngestionType_INGESTION_TYPE_APPEND:
		return filament.IngestionAppend
	case ingestionv1.IngestionType_INGESTION_TYPE_UPSERT:
		return filament.IngestionUpsert
	case ingestionv1.IngestionType_INGESTION_TYPE_DELETE:
		return filament.IngestionDelete
	case ingestionv1.IngestionType_INGESTION_TYPE_CDC:
		return filament.IngestionCDC
	default:
		return filament.IngestionSnapshotReplace
	}
}

func operationsToProto(ops []filament.Operation) []ingestionv1.Operation {
	out := make([]ingestionv1.Operation, 0, len(ops))
	for _, op := range ops {
		switch op {
		case filament.OpInsert:
			out = append(out, ingestionv1.Operation_OPERATION_INSERT)
		case filament.OpUpdate:
			out = append(out, ingestionv1.Operation_OPERATION_UPDATE)
		case filament.OpDelete:
			out = append(out, ingestionv1.Operation_OPERATION_DELETE)
		default:
			out = append(out, ingestionv1.Operation_OPERATION_UNSPECIFIED)
		}
	}
	return out
}

func writeModeToProto(mode filament.WriteMode) ingestionv1.WriteMode {
	switch mode {
	case filament.WriteAppend:
		return ingestionv1.WriteMode_WRITE_MODE_APPEND
	case filament.WriteReplace:
		return ingestionv1.WriteMode_WRITE_MODE_REPLACE
	case filament.WriteUpsert:
		return ingestionv1.WriteMode_WRITE_MODE_UPSERT
	case filament.WriteDelete:
		return ingestionv1.WriteMode_WRITE_MODE_DELETE
	case filament.WriteMerge:
		return ingestionv1.WriteMode_WRITE_MODE_MERGE
	default:
		return ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED
	}
}

func writeAtomicityToProto(atomicity filament.WriteAtomicity) ingestionv1.WriteAtomicity {
	switch atomicity {
	case filament.AtomicityBatch:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_BATCH
	case filament.AtomicityResource:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_RESOURCE
	case filament.AtomicityRun:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_RUN
	default:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_UNSPECIFIED
	}
}

func checkpointPolicyToProto(policy filament.CheckpointPolicy) ingestionv1.CheckpointPolicy {
	switch policy {
	case filament.CheckpointNone:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_NONE
	case filament.CheckpointAfterBatch:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_BATCH
	case filament.CheckpointAfterCommit:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_COMMIT
	default:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_UNSPECIFIED
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
	default:
		return ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func resourcesToProto(resources []filament.Resource) *ingestionv1.DiscoverResourcesResponse {
	out := make([]*ingestionv1.Resource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, &ingestionv1.Resource{
			Name:          resource.Name,
			Selectable:    resource.Selectable,
			PrimaryKey:    resource.PrimaryKey,
			EstimatedRows: resource.Estimated,
			Selector:      resource.Selector,
			DisplayName:   resource.DisplayName,
			Metadata:      resource.Metadata,
		})
	}
	return &ingestionv1.DiscoverResourcesResponse{Resources: out}
}

func runInfoToProto(state filament.RunState) *ingestionv1.RunInfo {
	return &ingestionv1.RunInfo{
		Run:     string(state.Run),
		Tenant:  string(state.Tenant),
		Status:  runStatusToProto(state.Status),
		Records: state.Records,
		Bytes:   state.Bytes,
		Error:   state.Error,
	}
}

func eventToProto(f events.Fact, replay bool) *ingestionv1.RunEvent {
	return &ingestionv1.RunEvent{
		Type:     f.Name,
		Tenant:   string(f.Tenant),
		Run:      string(f.Run),
		Resource: f.Resource,
		Seq:      f.Seq,
		AtUnixMs: f.At.UnixMilli(),
		Fields:   eventFieldsToProto(f.Data),
		Replay:   replay,
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
		Type:   runEventType(state.Status),
		Tenant: string(state.Tenant),
		Run:    string(state.Run),
		Fields: &ingestionv1.RunEventFields{
			Records: state.Records,
			Bytes:   state.Bytes,
			Error:   state.Error,
		},
		Replay: replay,
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

func validateConfigSchema(schema filament.ConfigSchema, cfg filament.Config) error {
	for _, field := range schema.Fields {
		if field.Required && !cfg.Has(field.Name) {
			return fmt.Errorf("%s is required", field.Name)
		}
	}
	return nil
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
		return "t1"
	}
	return tenant
}
