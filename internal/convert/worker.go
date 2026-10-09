package convert

import (
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/galaxy-io/filament"
	workerv1 "github.com/galaxy-io/filament/api/worker/v1"
)

// RefToProto names a connector for the worker.
func RefToProto(ref filament.ConnectorRef) *workerv1.Ref {
	return &workerv1.Ref{Kind: ConnectorKindToProto(ref.Kind), Name: ref.Name}
}

// RefFromProto decodes a connector name; an unspecified kind is an error.
func RefFromProto(ref *workerv1.Ref) (filament.ConnectorRef, error) {
	kind := ConnectorKindFromProto(ref.GetKind())
	if kind == filament.ConnectorKindUnspecified {
		return filament.ConnectorRef{}, fmt.Errorf("connector kind is required")
	}
	return filament.ConnectorRef{Kind: kind, Name: ref.GetName()}, nil
}

// SourceToProto encodes a full source spec: the public projection plus the
// engine facts it omits.
func SourceToProto(spec filament.ConnectorSpec) *workerv1.Source {
	out := &workerv1.Source{
		Spec:      SourceSpecToProto(spec),
		Policies:  sourcePoliciesToProto(spec.SourcePolicies),
		Resources: &workerv1.ResourceCapabilities{Discoverable: spec.Resources.Discoverable, PerResourceCursor: spec.Resources.PerResourceCursor},
	}
	for _, mode := range spec.Modes {
		out.ReadModes = append(out.ReadModes, ReadModeToProto(mode))
	}
	if spec.Stream != nil {
		out.Stream = &workerv1.StreamCapabilities{
			EmitsOps: operationsToProto(spec.Stream.EmitsOps),
			Input:    inputSemanticsToProto(spec.Stream.Input),
			Ordering: orderingsToProto(spec.Stream.Ordering),
			Delivery: deliveryToProto(spec.Stream.Delivery),
		}
	}
	return out
}

// SourceFromProto decodes a full source spec.
func SourceFromProto(source *workerv1.Source) (filament.ConnectorSpec, error) {
	spec, err := sourceSpecFromProto(source.GetSpec())
	if err != nil {
		return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", source.GetSpec().GetName(), err)
	}
	for _, mode := range source.GetReadModes() {
		decoded, err := ReadModeFromProto(mode)
		if err != nil {
			return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
		}
		spec.Modes = append(spec.Modes, decoded)
	}
	if spec.SourcePolicies, err = sourcePoliciesFromProto(source.GetPolicies()); err != nil {
		return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
	}
	spec.Resources = filament.ResourceCapabilities{
		Discoverable:      source.GetResources().GetDiscoverable(),
		PerResourceCursor: source.GetResources().GetPerResourceCursor(),
	}
	if stream := source.GetStream(); stream != nil {
		caps := &filament.StreamCapabilities{}
		if caps.EmitsOps, err = operationsFromProto(stream.GetEmitsOps()); err != nil {
			return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
		}
		if caps.Input, err = inputSemanticsFromProto(stream.GetInput()); err != nil {
			return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
		}
		if caps.Ordering, err = orderingsFromProto(stream.GetOrdering()); err != nil {
			return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
		}
		if caps.Delivery, err = deliveryFromProto(stream.GetDelivery()); err != nil {
			return filament.ConnectorSpec{}, fmt.Errorf("source %q: %w", spec.Name, err)
		}
		spec.Stream = caps
	}
	return spec, nil
}

func sourcePoliciesToProto(policies []filament.SourcePolicy) []*workerv1.SourcePolicy {
	out := make([]*workerv1.SourcePolicy, 0, len(policies))
	for _, policy := range policies {
		out = append(out, &workerv1.SourcePolicy{
			Mode:          ReadModeToProto(policy.Mode),
			EmitsOps:      operationsToProto(policy.EmitsOps),
			Ordered:       policy.Ordered,
			Checkpointing: checkpointPolicyToProto(policy.Checkpointing),
		})
	}
	return out
}

func sourcePoliciesFromProto(policies []*workerv1.SourcePolicy) ([]filament.SourcePolicy, error) {
	if len(policies) == 0 {
		return nil, nil
	}
	out := make([]filament.SourcePolicy, 0, len(policies))
	for _, policy := range policies {
		mode, err := ReadModeFromProto(policy.GetMode())
		if err != nil {
			return nil, err
		}
		ops, err := operationsFromProto(policy.GetEmitsOps())
		if err != nil {
			return nil, err
		}
		checkpointing, err := checkpointPolicyFromProto(policy.GetCheckpointing())
		if err != nil {
			return nil, err
		}
		out = append(out, filament.SourcePolicy{Mode: mode, EmitsOps: ops, Ordered: policy.GetOrdered(), Checkpointing: checkpointing})
	}
	return out, nil
}

// SinkToProto encodes a full sink spec: the public projection plus its
// capabilities.
func SinkToProto(spec filament.SinkSpec) *workerv1.Sink {
	caps := spec.Capabilities
	out := &workerv1.Sink{
		Spec: SinkSpecToProto(spec),
		Capabilities: &workerv1.SinkCapabilities{
			Transactional:       caps.Transactional,
			Schematized:         caps.Schematized,
			EncodedIntegrity:    caps.EncodedIntegrity,
			WritePolicies:       writePoliciesToProto(caps.WritePolicies),
			PreferredBatchRows:  int64(caps.PreferredBatchRows),
			PreferredBatchBytes: caps.PreferredBatchBytes,
		},
	}
	if caps.PreferredFlushInterval != 0 {
		out.Capabilities.PreferredFlushInterval = durationpb.New(caps.PreferredFlushInterval)
	}
	if caps.Stream != nil {
		stream := &workerv1.StreamingSinkCapabilities{
			WritePolicies:  writePoliciesToProto(caps.Stream.WritePolicies),
			OwnerFencing:   caps.Stream.OwnerFencing,
			IsolatedEpochs: caps.Stream.IsolatedEpochs,
		}
		if caps.Stream.InFlightBound != nil {
			stream.InFlightBound = durationpb.New(*caps.Stream.InFlightBound)
		}
		out.Capabilities.Stream = stream
	}
	return out
}

// SinkFromProto decodes a full sink spec.
func SinkFromProto(sink *workerv1.Sink) (filament.SinkSpec, error) {
	spec, err := sinkSpecFromProto(sink.GetSpec())
	if err != nil {
		return filament.SinkSpec{}, fmt.Errorf("sink %q: %w", sink.GetSpec().GetName(), err)
	}
	caps := sink.GetCapabilities()
	spec.Capabilities = filament.SinkCapabilities{
		Transactional:       caps.GetTransactional(),
		Schematized:         caps.GetSchematized(),
		EncodedIntegrity:    caps.GetEncodedIntegrity(),
		PreferredBatchRows:  int(caps.GetPreferredBatchRows()),
		PreferredBatchBytes: caps.GetPreferredBatchBytes(),
	}
	if caps.GetPreferredFlushInterval() != nil {
		spec.Capabilities.PreferredFlushInterval = caps.GetPreferredFlushInterval().AsDuration()
	}
	if spec.Capabilities.WritePolicies, err = writePoliciesFromProto(caps.GetWritePolicies()); err != nil {
		return filament.SinkSpec{}, fmt.Errorf("sink %q: %w", spec.Name, err)
	}
	if stream := caps.GetStream(); stream != nil {
		decoded := &filament.StreamingSinkCapabilities{OwnerFencing: stream.GetOwnerFencing(), IsolatedEpochs: stream.GetIsolatedEpochs()}
		if decoded.WritePolicies, err = writePoliciesFromProto(stream.GetWritePolicies()); err != nil {
			return filament.SinkSpec{}, fmt.Errorf("sink %q: %w", spec.Name, err)
		}
		if stream.InFlightBound != nil {
			bound := stream.GetInFlightBound().AsDuration()
			decoded.InFlightBound = &bound
		}
		spec.Capabilities.Stream = decoded
	}
	return spec, nil
}

func writePoliciesToProto(policies []filament.WritePolicyCapability) []*workerv1.WritePolicyCapability {
	out := make([]*workerv1.WritePolicyCapability, 0, len(policies))
	for _, policy := range policies {
		out = append(out, &workerv1.WritePolicyCapability{
			Mode:          WriteModeToProto(policy.Mode),
			RequiresPk:    policy.RequiresPK,
			RequiresOrder: policy.RequiresOrder,
			AcceptsOps:    operationsToProto(policy.AcceptsOps),
			Atomicity:     atomicityToProto(policy.Atomicity),
			Durability:    durabilityToProto(policy.Durability),
		})
	}
	return out
}

func writePoliciesFromProto(policies []*workerv1.WritePolicyCapability) ([]filament.WritePolicyCapability, error) {
	if len(policies) == 0 {
		return nil, nil
	}
	out := make([]filament.WritePolicyCapability, 0, len(policies))
	for _, policy := range policies {
		mode, err := WriteModeFromProto(policy.GetMode())
		if err != nil {
			return nil, err
		}
		ops, err := operationsFromProto(policy.GetAcceptsOps())
		if err != nil {
			return nil, err
		}
		atomicity, err := atomicityFromProto(policy.GetAtomicity())
		if err != nil {
			return nil, err
		}
		durability, err := durabilityFromProto(policy.GetDurability())
		if err != nil {
			return nil, err
		}
		out = append(out, filament.WritePolicyCapability{
			Mode: mode, RequiresPK: policy.GetRequiresPk(), RequiresOrder: policy.GetRequiresOrder(),
			AcceptsOps: ops, Atomicity: atomicity, Durability: durability,
		})
	}
	return out, nil
}

// InspectionsToProto encodes per-resource inspections.
func InspectionsToProto(inspections []filament.Inspection) []*workerv1.Inspection {
	out := make([]*workerv1.Inspection, 0, len(inspections))
	for _, inspection := range inspections {
		out = append(out, &workerv1.Inspection{
			Name:               inspection.Name,
			Status:             inspectStatusToProto(inspection.Status),
			Message:            inspection.Message,
			PrimaryKey:         inspection.PrimaryKey,
			Columns:            CursorColumnsToProto(inspection.Columns),
			Ranked:             inspection.Ranked,
			ManagedIncremental: inspection.ManagedIncremental,
		})
	}
	return out
}

// InspectionsFromProto decodes per-resource inspections.
func InspectionsFromProto(inspections []*workerv1.Inspection) ([]filament.Inspection, error) {
	out := make([]filament.Inspection, 0, len(inspections))
	for _, inspection := range inspections {
		status, err := inspectStatusFromProto(inspection.GetStatus())
		if err != nil {
			return nil, fmt.Errorf("resource %q: %w", inspection.GetName(), err)
		}
		out = append(out, filament.Inspection{
			Name:               inspection.GetName(),
			Status:             status,
			Message:            inspection.GetMessage(),
			PrimaryKey:         inspection.GetPrimaryKey(),
			Columns:            CursorColumnsFromProto(inspection.GetColumns()),
			Ranked:             inspection.GetRanked(),
			ManagedIncremental: inspection.GetManagedIncremental(),
		})
	}
	return out, nil
}

// ReplicationStreamPlanToProto encodes a plan; its config maps become Structs.
func ReplicationStreamPlanToProto(plan filament.ReplicationStreamPlan) (*workerv1.ReplicationStreamPlan, error) {
	consumer, err := StructFromMap(plan.ConsumerConfig)
	if err != nil {
		return nil, fmt.Errorf("consumer config: %w", err)
	}
	continuity, err := StructFromMap(plan.ContinuityConfig)
	if err != nil {
		return nil, fmt.Errorf("continuity config: %w", err)
	}
	return &workerv1.ReplicationStreamPlan{
		Resources:        plan.Resources,
		ConsumerName:     plan.ConsumerName,
		ConsumerConfig:   consumer,
		ContinuityConfig: continuity,
	}, nil
}

// ReplicationStreamPlanFromProto decodes a plan. Absent config maps decode as
// nil so an empty plan round-trips to its zero value.
func ReplicationStreamPlanFromProto(plan *workerv1.ReplicationStreamPlan) filament.ReplicationStreamPlan {
	out := filament.ReplicationStreamPlan{Resources: plan.GetResources(), ConsumerName: plan.GetConsumerName()}
	if len(plan.GetConsumerConfig().GetFields()) > 0 {
		out.ConsumerConfig = plan.GetConsumerConfig().AsMap()
	}
	if len(plan.GetContinuityConfig().GetFields()) > 0 {
		out.ContinuityConfig = plan.GetContinuityConfig().AsMap()
	}
	return out
}
