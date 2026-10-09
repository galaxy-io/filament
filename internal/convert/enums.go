package convert

import (
	"fmt"

	"github.com/galaxy-io/filament"
	workerv1 "github.com/galaxy-io/filament/api/worker/v1"
)

// Encoders map an unknown value to unspecified; decoders reject unspecified.
// A spec that names a value the enum lacks therefore fails loudly at the
// first decode instead of silently changing meaning.

func operationsToProto(ops []filament.Operation) []workerv1.Operation {
	out := make([]workerv1.Operation, 0, len(ops))
	for _, op := range ops {
		out = append(out, operationToProto(op))
	}
	return out
}

func operationsFromProto(ops []workerv1.Operation) ([]filament.Operation, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	out := make([]filament.Operation, 0, len(ops))
	for _, op := range ops {
		decoded, err := operationFromProto(op)
		if err != nil {
			return nil, err
		}
		out = append(out, decoded)
	}
	return out, nil
}

func operationToProto(op filament.Operation) workerv1.Operation {
	switch op {
	case filament.OpInsert:
		return workerv1.Operation_OPERATION_INSERT
	case filament.OpUpdate:
		return workerv1.Operation_OPERATION_UPDATE
	case filament.OpDelete:
		return workerv1.Operation_OPERATION_DELETE
	default:
		return workerv1.Operation_OPERATION_UNSPECIFIED
	}
}

func operationFromProto(op workerv1.Operation) (filament.Operation, error) {
	switch op {
	case workerv1.Operation_OPERATION_INSERT:
		return filament.OpInsert, nil
	case workerv1.Operation_OPERATION_UPDATE:
		return filament.OpUpdate, nil
	case workerv1.Operation_OPERATION_DELETE:
		return filament.OpDelete, nil
	default:
		return 0, fmt.Errorf("unknown operation %d", op)
	}
}

func checkpointPolicyToProto(policy filament.CheckpointPolicy) workerv1.CheckpointPolicy {
	switch policy {
	case filament.CheckpointNone:
		return workerv1.CheckpointPolicy_CHECKPOINT_POLICY_NONE
	case filament.CheckpointAfterBatch:
		return workerv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_BATCH
	case filament.CheckpointAfterCommit:
		return workerv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_COMMIT
	default:
		return workerv1.CheckpointPolicy_CHECKPOINT_POLICY_UNSPECIFIED
	}
}

func checkpointPolicyFromProto(policy workerv1.CheckpointPolicy) (filament.CheckpointPolicy, error) {
	switch policy {
	case workerv1.CheckpointPolicy_CHECKPOINT_POLICY_NONE:
		return filament.CheckpointNone, nil
	case workerv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_BATCH:
		return filament.CheckpointAfterBatch, nil
	case workerv1.CheckpointPolicy_CHECKPOINT_POLICY_AFTER_COMMIT:
		return filament.CheckpointAfterCommit, nil
	default:
		return "", fmt.Errorf("unknown checkpoint policy %d", policy)
	}
}

func inputSemanticsToProto(input filament.InputSemantics) workerv1.InputSemantics {
	switch input {
	case filament.InputRows:
		return workerv1.InputSemantics_INPUT_SEMANTICS_ROWS
	case filament.InputChanges:
		return workerv1.InputSemantics_INPUT_SEMANTICS_CHANGES
	case filament.InputMessages:
		return workerv1.InputSemantics_INPUT_SEMANTICS_MESSAGES
	default:
		return workerv1.InputSemantics_INPUT_SEMANTICS_UNSPECIFIED
	}
}

func inputSemanticsFromProto(input workerv1.InputSemantics) (filament.InputSemantics, error) {
	switch input {
	case workerv1.InputSemantics_INPUT_SEMANTICS_ROWS:
		return filament.InputRows, nil
	case workerv1.InputSemantics_INPUT_SEMANTICS_CHANGES:
		return filament.InputChanges, nil
	case workerv1.InputSemantics_INPUT_SEMANTICS_MESSAGES:
		return filament.InputMessages, nil
	default:
		return "", fmt.Errorf("unknown input semantics %d", input)
	}
}

func orderingsToProto(orderings []filament.Ordering) []workerv1.Ordering {
	out := make([]workerv1.Ordering, 0, len(orderings))
	for _, ordering := range orderings {
		out = append(out, orderingToProto(ordering))
	}
	return out
}

func orderingsFromProto(orderings []workerv1.Ordering) ([]filament.Ordering, error) {
	if len(orderings) == 0 {
		return nil, nil
	}
	out := make([]filament.Ordering, 0, len(orderings))
	for _, ordering := range orderings {
		decoded, err := orderingFromProto(ordering)
		if err != nil {
			return nil, err
		}
		out = append(out, decoded)
	}
	return out, nil
}

func orderingToProto(ordering filament.Ordering) workerv1.Ordering {
	switch ordering {
	case filament.OrderingNone:
		return workerv1.Ordering_ORDERING_NONE
	case filament.OrderingPartition:
		return workerv1.Ordering_ORDERING_PARTITION
	case filament.OrderingTransaction:
		return workerv1.Ordering_ORDERING_TRANSACTION
	case filament.OrderingGlobalStrict:
		return workerv1.Ordering_ORDERING_GLOBAL_STRICT
	default:
		return workerv1.Ordering_ORDERING_UNSPECIFIED
	}
}

func orderingFromProto(ordering workerv1.Ordering) (filament.Ordering, error) {
	switch ordering {
	case workerv1.Ordering_ORDERING_NONE:
		return filament.OrderingNone, nil
	case workerv1.Ordering_ORDERING_PARTITION:
		return filament.OrderingPartition, nil
	case workerv1.Ordering_ORDERING_TRANSACTION:
		return filament.OrderingTransaction, nil
	case workerv1.Ordering_ORDERING_GLOBAL_STRICT:
		return filament.OrderingGlobalStrict, nil
	default:
		return "", fmt.Errorf("unknown ordering %d", ordering)
	}
}

func deliveryToProto(delivery filament.DeliveryGuarantee) workerv1.DeliveryGuarantee {
	switch delivery {
	case filament.DeliveryReplayableAtLeastOnce:
		return workerv1.DeliveryGuarantee_DELIVERY_GUARANTEE_REPLAYABLE_AT_LEAST_ONCE
	case filament.DeliveryBestEffort:
		return workerv1.DeliveryGuarantee_DELIVERY_GUARANTEE_BEST_EFFORT
	default:
		return workerv1.DeliveryGuarantee_DELIVERY_GUARANTEE_UNSPECIFIED
	}
}

func deliveryFromProto(delivery workerv1.DeliveryGuarantee) (filament.DeliveryGuarantee, error) {
	switch delivery {
	case workerv1.DeliveryGuarantee_DELIVERY_GUARANTEE_REPLAYABLE_AT_LEAST_ONCE:
		return filament.DeliveryReplayableAtLeastOnce, nil
	case workerv1.DeliveryGuarantee_DELIVERY_GUARANTEE_BEST_EFFORT:
		return filament.DeliveryBestEffort, nil
	default:
		return "", fmt.Errorf("unknown delivery guarantee %d", delivery)
	}
}

func atomicityToProto(atomicity filament.WriteAtomicity) workerv1.WriteAtomicity {
	switch atomicity {
	case filament.AtomicityRecord:
		return workerv1.WriteAtomicity_WRITE_ATOMICITY_RECORD
	case filament.AtomicityBatch:
		return workerv1.WriteAtomicity_WRITE_ATOMICITY_BATCH
	case filament.AtomicityResource:
		return workerv1.WriteAtomicity_WRITE_ATOMICITY_RESOURCE
	case filament.AtomicityRun:
		return workerv1.WriteAtomicity_WRITE_ATOMICITY_RUN
	default:
		return workerv1.WriteAtomicity_WRITE_ATOMICITY_UNSPECIFIED
	}
}

func atomicityFromProto(atomicity workerv1.WriteAtomicity) (filament.WriteAtomicity, error) {
	switch atomicity {
	case workerv1.WriteAtomicity_WRITE_ATOMICITY_RECORD:
		return filament.AtomicityRecord, nil
	case workerv1.WriteAtomicity_WRITE_ATOMICITY_BATCH:
		return filament.AtomicityBatch, nil
	case workerv1.WriteAtomicity_WRITE_ATOMICITY_RESOURCE:
		return filament.AtomicityResource, nil
	case workerv1.WriteAtomicity_WRITE_ATOMICITY_RUN:
		return filament.AtomicityRun, nil
	default:
		return "", fmt.Errorf("unknown write atomicity %d", atomicity)
	}
}

func durabilityToProto(durability filament.WriteDurability) workerv1.WriteDurability {
	switch durability {
	case filament.DurabilityAfterApply:
		return workerv1.WriteDurability_WRITE_DURABILITY_AFTER_APPLY
	case filament.DurabilityAfterCommit:
		return workerv1.WriteDurability_WRITE_DURABILITY_AFTER_COMMIT
	default:
		return workerv1.WriteDurability_WRITE_DURABILITY_UNSPECIFIED
	}
}

func durabilityFromProto(durability workerv1.WriteDurability) (filament.WriteDurability, error) {
	switch durability {
	case workerv1.WriteDurability_WRITE_DURABILITY_AFTER_APPLY:
		return filament.DurabilityAfterApply, nil
	case workerv1.WriteDurability_WRITE_DURABILITY_AFTER_COMMIT:
		return filament.DurabilityAfterCommit, nil
	default:
		return "", fmt.Errorf("unknown write durability %d", durability)
	}
}

func inspectStatusToProto(status filament.InspectStatus) workerv1.InspectStatus {
	switch status {
	case filament.InspectOK:
		return workerv1.InspectStatus_INSPECT_STATUS_OK
	case filament.InspectUnsupported:
		return workerv1.InspectStatus_INSPECT_STATUS_UNSUPPORTED
	case filament.InspectFailed:
		return workerv1.InspectStatus_INSPECT_STATUS_FAILED
	default:
		return workerv1.InspectStatus_INSPECT_STATUS_UNSPECIFIED
	}
}

func inspectStatusFromProto(status workerv1.InspectStatus) (filament.InspectStatus, error) {
	switch status {
	case workerv1.InspectStatus_INSPECT_STATUS_OK:
		return filament.InspectOK, nil
	case workerv1.InspectStatus_INSPECT_STATUS_UNSUPPORTED:
		return filament.InspectUnsupported, nil
	case workerv1.InspectStatus_INSPECT_STATUS_FAILED:
		return filament.InspectFailed, nil
	default:
		return 0, fmt.Errorf("unknown inspect status %d", status)
	}
}
