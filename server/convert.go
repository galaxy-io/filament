package server

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/events"
)

func sourceSpecToProto(spec ingestion.ConnectorSpec) *ingestionv1.ProviderSpec {
	return &ingestionv1.ProviderSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Kind:         ingestionv1.ProviderKind_PROVIDER_KIND_SOURCE,
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

func sinkSpecToProto(spec ingestion.SinkSpec) *ingestionv1.ProviderSpec {
	return &ingestionv1.ProviderSpec{
		Name:         spec.Name,
		DisplayName:  spec.DisplayName,
		Kind:         ingestionv1.ProviderKind_PROVIDER_KIND_SINK,
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

func configSchemaToProto(schema ingestion.ConfigSchema) *ingestionv1.ConfigSchema {
	fields := make([]*ingestionv1.ConfigField, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		fields = append(fields, &ingestionv1.ConfigField{
			Name:     field.Name,
			Type:     fieldTypeToProto(field.Type),
			Required: field.Required,
			Default:  valueToProto(field.Default),
			Enum:     field.Enum,
			Help:     field.Help,
		})
	}
	return &ingestionv1.ConfigSchema{Fields: fields}
}

func sourcePoliciesToProto(policies []ingestion.SourcePolicy) []*ingestionv1.SourcePolicy {
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

func writePolicyCapabilitiesToProto(caps []ingestion.WritePolicyCapability) []*ingestionv1.WritePolicyCapability {
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

func modesToProto(modes []ingestion.ReplicationMode) []ingestionv1.ReplicationMode {
	out := make([]ingestionv1.ReplicationMode, 0, len(modes))
	for _, mode := range modes {
		out = append(out, modeToProto(mode))
	}
	return out
}

func modeToProto(mode ingestion.ReplicationMode) ingestionv1.ReplicationMode {
	switch mode {
	case ingestion.ModeFull:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_FULL
	case ingestion.ModeIncremental:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_INCREMENTAL
	case ingestion.ModeCDC:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_CDC
	default:
		return ingestionv1.ReplicationMode_REPLICATION_MODE_UNSPECIFIED
	}
}

func ingestionTypeFromProto(t ingestionv1.IngestionType) ingestion.IngestionType {
	switch t {
	case ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT:
		return ingestion.IngestionSnapshotUpsert
	case ingestionv1.IngestionType_INGESTION_TYPE_APPEND:
		return ingestion.IngestionAppend
	case ingestionv1.IngestionType_INGESTION_TYPE_UPSERT:
		return ingestion.IngestionUpsert
	case ingestionv1.IngestionType_INGESTION_TYPE_DELETE:
		return ingestion.IngestionDelete
	case ingestionv1.IngestionType_INGESTION_TYPE_CDC:
		return ingestion.IngestionCDC
	default:
		return ingestion.IngestionSnapshotReplace
	}
}

func operationsToProto(ops []ingestion.Operation) []ingestionv1.Operation {
	out := make([]ingestionv1.Operation, 0, len(ops))
	for _, op := range ops {
		switch op {
		case ingestion.OpInsert:
			out = append(out, ingestionv1.Operation_OPERATION_INSERT)
		case ingestion.OpUpdate:
			out = append(out, ingestionv1.Operation_OPERATION_UPDATE)
		case ingestion.OpDelete:
			out = append(out, ingestionv1.Operation_OPERATION_DELETE)
		default:
			out = append(out, ingestionv1.Operation_OPERATION_UNSPECIFIED)
		}
	}
	return out
}

func writeModeToProto(mode ingestion.WriteMode) ingestionv1.WriteMode {
	switch mode {
	case ingestion.WriteAppend:
		return ingestionv1.WriteMode_WRITE_MODE_APPEND
	case ingestion.WriteReplace:
		return ingestionv1.WriteMode_WRITE_MODE_REPLACE
	case ingestion.WriteUpsert:
		return ingestionv1.WriteMode_WRITE_MODE_UPSERT
	case ingestion.WriteDelete:
		return ingestionv1.WriteMode_WRITE_MODE_DELETE
	case ingestion.WriteMerge:
		return ingestionv1.WriteMode_WRITE_MODE_MERGE
	default:
		return ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED
	}
}

func writeAtomicityToProto(atomicity ingestion.WriteAtomicity) ingestionv1.WriteAtomicity {
	switch atomicity {
	case ingestion.AtomicityBatch:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_BATCH
	case ingestion.AtomicityResource:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_RESOURCE
	case ingestion.AtomicityRun:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_RUN
	default:
		return ingestionv1.WriteAtomicity_WRITE_ATOMICITY_UNSPECIFIED
	}
}

func checkpointPolicyToProto(policy ingestion.CheckpointPolicy) ingestionv1.CheckpointPolicy {
	switch policy {
	case ingestion.CheckpointNone:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_NONE
	case ingestion.CheckpointAfterBatch:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_BATCH
	case ingestion.CheckpointAfterCommit:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_COMMIT
	default:
		return ingestionv1.CheckpointPolicy_CHECKPOINT_POLICY_UNSPECIFIED
	}
}

func fieldTypeToProto(t ingestion.FieldType) ingestionv1.FieldType {
	switch t {
	case ingestion.FieldString:
		return ingestionv1.FieldType_FIELD_TYPE_STRING
	case ingestion.FieldInt:
		return ingestionv1.FieldType_FIELD_TYPE_INT
	case ingestion.FieldBool:
		return ingestionv1.FieldType_FIELD_TYPE_BOOL
	case ingestion.FieldSecret:
		return ingestionv1.FieldType_FIELD_TYPE_SECRET
	case ingestion.FieldDuration:
		return ingestionv1.FieldType_FIELD_TYPE_DURATION
	case ingestion.FieldEnum:
		return ingestionv1.FieldType_FIELD_TYPE_ENUM
	case ingestion.FieldObject:
		return ingestionv1.FieldType_FIELD_TYPE_OBJECT
	default:
		return ingestionv1.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}

func runStatusToProto(status ingestion.RunStatus) ingestionv1.RunStatus {
	switch status {
	case ingestion.RunRequested:
		return ingestionv1.RunStatus_RUN_STATUS_REQUESTED
	case ingestion.RunRunning:
		return ingestionv1.RunStatus_RUN_STATUS_RUNNING
	case ingestion.RunCompleted:
		return ingestionv1.RunStatus_RUN_STATUS_COMPLETED
	case ingestion.RunFailed:
		return ingestionv1.RunStatus_RUN_STATUS_FAILED
	case ingestion.RunCanceled:
		return ingestionv1.RunStatus_RUN_STATUS_CANCELED
	case ingestion.RunPaused:
		return ingestionv1.RunStatus_RUN_STATUS_PAUSED
	case ingestion.RunPartial:
		return ingestionv1.RunStatus_RUN_STATUS_PARTIAL
	default:
		return ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func resourcesToProto(resources []ingestion.Resource) *ingestionv1.DiscoverResourcesResponse {
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

func runInfoToProto(state ingestion.RunState) *ingestionv1.RunInfo {
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

func runSnapshotEvent(state ingestion.RunState, replay bool) *ingestionv1.RunEvent {
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

func runStatusTerminal(status ingestion.RunStatus) bool {
	switch status {
	case ingestion.RunCompleted, ingestion.RunFailed, ingestion.RunCanceled, ingestion.RunPartial:
		return true
	default:
		return false
	}
}

func runEventType(status ingestion.RunStatus) string {
	switch status {
	case ingestion.RunRequested:
		return events.RunRequested.Name()
	case ingestion.RunRunning:
		return events.RunStarted.Name()
	case ingestion.RunCompleted:
		return events.RunCompleted.Name()
	case ingestion.RunFailed:
		return events.RunFailed.Name()
	case ingestion.RunCanceled:
		return "run.canceled"
	case ingestion.RunPaused:
		return "run.paused"
	case ingestion.RunPartial:
		return events.RunPartial.Name()
	default:
		return "unspecified"
	}
}

func runStatusEventType(status ingestion.RunStatus) string {
	switch status {
	case ingestion.RunRequested:
		return events.RunRequested.Name()
	case ingestion.RunRunning:
		return events.ResourceStarted.Name()
	case ingestion.RunCompleted:
		return events.ResourceCompleted.Name()
	case ingestion.RunFailed:
		return events.ResourceFailed.Name()
	case ingestion.RunCanceled:
		return "run.canceled"
	case ingestion.RunPaused:
		return "run.paused"
	case ingestion.RunPartial:
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

func validateConfigSchema(schema ingestion.ConfigSchema, cfg ingestion.Config) error {
	for _, field := range schema.Fields {
		if field.Required && !cfg.Has(field.Name) {
			return fmt.Errorf("%s is required", field.Name)
		}
	}
	return nil
}

func defaultTenant(tenant string) string {
	if tenant == "" {
		return "t1"
	}
	return tenant
}
