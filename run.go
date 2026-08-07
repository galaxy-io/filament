package filament

import (
	"fmt"
	"time"
)

// RunSpec is the fully resolved execution plan for one run — what a Runtime
// receives after the engine has bound refs, ingestion type, and options.
type RunSpec struct {
	Tenant            TenantID
	Run               RunID
	PipelineID        string
	PipelineVersionID int64
	CheckpointRoute   string
	CursorConfigs     map[string]ResourceCursorConfig
	Source            Ref
	Sink              Ref
	Resources         []string
	Selectors         []string
	IngestionType     IngestionType
	Mode              ReplicationMode
	Checkpoint        *CheckpointData
	Options           RunOptions
}

// RunRequest is the caller-facing ask for a run, deduplicated by
// IdempotencyKey; the engine resolves it into a RunSpec.
type RunRequest struct {
	Tenant             TenantID
	PipelineID         string
	PipelineVersionID  int64
	IdempotencyKey     string
	Source             Ref
	Sink               Ref
	SourceConnectionID string
	SinkConnectionID   string
	Resources          []string
	Selectors          []string
	IngestionType      IngestionType
	CheckpointRoute    string
	CursorConfigs      map[string]ResourceCursorConfig
	Options            RunOptions
	ScheduleID         ScheduleID
}

// ResourceCursorConfig selects one resource's durable incremental field and
// optional overlap window for late transactions.
type ResourceCursorConfig struct {
	Field           string
	LookbackSeconds int64
}

// ResourceCheckpointKey returns the stable cross-run key for resource. False
// means the request did not originate from a versioned pipeline route.
func (r RunRequest) ResourceCheckpointKey(resource string) (ResourceCheckpointKey, bool) {
	if r.PipelineID == "" || r.PipelineVersionID <= 0 || r.CheckpointRoute == "" || resource == "" {
		return ResourceCheckpointKey{}, false
	}
	return ResourceCheckpointKey{
		PipelineID: r.PipelineID, PipelineVersionID: r.PipelineVersionID,
		Route: r.CheckpointRoute, Resource: resource,
	}, true
}

// ResourceCheckpointKey returns the stable cross-run key for resource.
func (s RunSpec) ResourceCheckpointKey(resource string) (ResourceCheckpointKey, bool) {
	return RunRequest{
		PipelineID: s.PipelineID, PipelineVersionID: s.PipelineVersionID,
		CheckpointRoute: s.CheckpointRoute,
	}.ResourceCheckpointKey(resource)
}

// RunOptions tunes throughput knobs for a run; zero values defer to engine
// defaults.
type RunOptions struct {
	FetchSize           int
	BatchMaxRows        int
	BatchMaxBytes       int64
	RateLimit           *RatePolicy
	SnapshotParallelism int

	// CheckpointEvery is how many written batches advance the cursor before it is
	// persisted (0 → DefaultCheckpointEvery). Larger values cut DataStore writes at
	// the cost of more redundant re-reads on resume (harmless: the sink is idempotent).
	CheckpointEvery int
}

// DefaultCheckpointEvery persists a resource's cursor every N written batches when
// RunOptions.CheckpointEvery is unset.
const DefaultCheckpointEvery = 25

// RunState is the persisted record of a run: its request, per-resource
// progress, and terminal outcome.
type RunState struct {
	Run        RunID
	Tenant     TenantID
	Status     RunStatus
	Request    RunRequest
	Resources  []ResourceState
	Records    int64
	Bytes      int64
	StartedAt  time.Time
	FinishedAt *time.Time
	Error      string
	ScheduleID ScheduleID
}

// RunStatus is the lifecycle state of a run or resource.
type RunStatus int

// Run lifecycle states, in rough order of progression.
const (
	RunRequested RunStatus = iota
	RunRunning
	RunCompleted
	RunFailed
	RunCanceled
	RunPaused
	// RunPartial is a run that failed mid-extract but persisted resumable progress.
	// Unlike RunFailed it is not terminal: re-emitting run.requested for the same
	// RunID resumes it from the last checkpoint. Only resumable runs reach it.
	RunPartial
)

// RunResult is the terminal outcome of a run as reported by a RunHandle.
type RunResult struct {
	Status  RunStatus
	Records int64
	Bytes   int64
	URIs    []string
	Error   string
}

// ResourceState is one resource's persisted progress within a run.
type ResourceState struct {
	Run        RunID // owning run — the key a DataStore files this under
	Tenant     TenantID
	Resource   string
	Enabled    bool
	Status     RunStatus
	Records    int64
	Bytes      int64
	Checkpoint *CheckpointData
	Error      string
}

// RunFilter narrows a DataStore run listing; zero fields match everything.
// Since is inclusive and Until exclusive on StartedAt; either bound excludes
// runs that never started.
type RunFilter struct {
	Tenant            TenantID
	PipelineID        string
	PipelineVersionID *int64
	Source            string
	Status            []RunStatus
	Schedule          ScheduleID
	Since             time.Time
	Until             time.Time
	Limit             int
	Offset            int
	Cursor            string
}

// SyncSnapshot is a consistent read of a run and its resources at bus
// sequence AtSeq, the anchor a live tail resumes from.
type SyncSnapshot struct {
	Run       RunState
	Resources []ResourceState
	AtSeq     uint64
}

// IngestionType names how a run moves data end to end; it determines both the
// source's read policy and the sink's write policy.
type IngestionType string

// The defined ingestion types, named {read}_{write}: what the source reads
// crossed with how the sink lands it. CDC implies both sides.
const (
	IngestionSnapshotReplace   IngestionType = "snapshot_replace"
	IngestionSnapshotUpsert    IngestionType = "snapshot_upsert"
	IngestionSnapshotAppend    IngestionType = "snapshot_append"
	IngestionIncrementalAppend IngestionType = "incremental_append"
	IngestionIncrementalUpsert IngestionType = "incremental_upsert"
	IngestionIncrementalDelete IngestionType = "incremental_delete"
	IngestionCDC               IngestionType = "cdc"
)

// OrDefault substitutes IngestionSnapshotReplace for the empty type.
func (t IngestionType) OrDefault() IngestionType {
	if t == "" {
		return IngestionSnapshotReplace
	}
	return t
}

// WritePolicy returns the sink-side policy this ingestion type implies.
func (t IngestionType) WritePolicy() WritePolicy {
	return WritePolicyForIngestion(t)
}

// WriteCapability returns the capability a sink must offer to serve this type.
func (t IngestionType) WriteCapability() WritePolicyCapability {
	return t.WritePolicy().Capability
}

// SourcePolicy returns the source-side policy this ingestion type implies.
func (t IngestionType) SourcePolicy() SourcePolicy {
	return SourcePolicyForIngestion(t)
}

// WriteCapabilities maps each ingestion type to its required sink capability,
// preserving order — the shape connector Specs advertise.
func WriteCapabilities(types ...IngestionType) []WritePolicyCapability {
	out := make([]WritePolicyCapability, 0, len(types))
	for _, t := range types {
		out = append(out, t.WriteCapability())
	}
	return out
}

// SourcePolicies maps each ingestion type to its source policy, preserving
// order — the shape connector Specs advertise.
func SourcePolicies(types ...IngestionType) []SourcePolicy {
	out := make([]SourcePolicy, 0, len(types))
	for _, t := range types {
		out = append(out, t.SourcePolicy())
	}
	return out
}

// WriteMode is how a sink lands records: append, replace, upsert, delete, or
// CDC merge.
type WriteMode string

// The defined write modes.
const (
	WriteAppend       WriteMode = "append"
	WriteReplace      WriteMode = "replace"
	WriteUpsert       WriteMode = "upsert"
	WriteDelete       WriteMode = "delete"
	WriteMerge        WriteMode = "merge"
	WriteAppendDedupe WriteMode = "append_dedupe"
)

// IngestionFor compiles the two user levers — per-table read mode and sink
// write mode — into the internal ingestion type. Zero levers default to a
// full-refresh replace; an unset write on an incremental read defaults to
// upsert. CDC connections never reach this: their edges are always
// IngestionCDC.
func IngestionFor(read ReplicationMode, write WriteMode) (IngestionType, error) {
	if write == "" {
		if read == ModeIncremental {
			write = WriteUpsert
		} else {
			write = WriteReplace
		}
	}
	if write == WriteAppendDedupe {
		return "", fmt.Errorf("write mode %q is not supported yet", write)
	}
	switch read {
	case ModeFull:
		switch write {
		case WriteReplace:
			return IngestionSnapshotReplace, nil
		case WriteUpsert:
			return IngestionSnapshotUpsert, nil
		case WriteAppend:
			return IngestionSnapshotAppend, nil
		}
	case ModeIncremental:
		switch write {
		case WriteAppend:
			return IngestionIncrementalAppend, nil
		case WriteUpsert:
			return IngestionIncrementalUpsert, nil
		case WriteDelete:
			return IngestionIncrementalDelete, nil
		}
	}
	return "", fmt.Errorf("read mode %v cannot combine with write mode %q", read, write)
}

// WriteAtomicity is the unit at which a sink's writes become visible.
type WriteAtomicity string

// Atomicity units, smallest to largest.
const (
	AtomicityBatch    WriteAtomicity = "batch"
	AtomicityResource WriteAtomicity = "resource"
	AtomicityRun      WriteAtomicity = "run"
)

// CheckpointPolicy is when a cursor may be persisted relative to writes.
type CheckpointPolicy string

// Checkpoint timings, weakest to strongest.
const (
	CheckpointNone        CheckpointPolicy = "none"
	CheckpointAfterBatch  CheckpointPolicy = "after_batch"
	CheckpointAfterCommit CheckpointPolicy = "after_commit"
)

// WritePolicyCapability is what a sink must support to serve a write mode:
// key/order requirements, accepted operations, and atomicity.
type WritePolicyCapability struct {
	Mode          WriteMode
	RequiresPK    bool
	RequiresOrder bool
	AcceptsOps    []Operation
	Atomicity     WriteAtomicity
}

// WritePolicy binds a capability to one resource's keys and checkpoint timing
// — the per-resource contract handed to a sink via ApplyOptions.
type WritePolicy struct {
	Capability WritePolicyCapability
	Resource   string
	Keys       []string
	Checkpoint CheckpointPolicy
}

// Accepts reports whether the capability admits op; an empty AcceptsOps
// admits everything.
func (c WritePolicyCapability) Accepts(op Operation) bool {
	if len(c.AcceptsOps) == 0 {
		return true
	}
	for _, candidate := range c.AcceptsOps {
		if candidate == op {
			return true
		}
	}
	return false
}

// ValidateRecords rejects the first record whose operation the policy does
// not accept.
func (p WritePolicy) ValidateRecords(resource string, records []Record) error {
	for _, rec := range records {
		if !p.Capability.Accepts(rec.Op) {
			return fmt.Errorf("write policy %q does not accept %s record for resource %q", p.Capability.Mode, OperationName(rec.Op), resource)
		}
	}
	return nil
}

// OperationName renders an Operation for error messages and logs.
func OperationName(op Operation) string {
	switch op {
	case OpInsert:
		return "insert"
	case OpUpdate:
		return "update"
	case OpDelete:
		return "delete"
	default:
		return fmt.Sprintf("operation(%d)", op)
	}
}

// SourcePolicy is the read-side contract an ingestion type implies: mode,
// emitted operations, ordering, and checkpoint timing.
type SourcePolicy struct {
	Mode          ReplicationMode
	EmitsOps      []Operation
	Ordered       bool
	Checkpointing CheckpointPolicy
}

// IngestionPlan is the resolved policy set for a run: one source policy plus
// per-resource write policies.
type IngestionPlan struct {
	Type          IngestionType
	SourcePolicy  SourcePolicy
	WritePolicies map[string]WritePolicy
	RequiresCDC   bool
	RequiresPK    bool
}

// WritePolicyForIngestion derives the canonical sink-side policy for an
// ingestion type; keys and resource are bound later, per resource.
func WritePolicyForIngestion(t IngestionType) WritePolicy {
	capability := WritePolicyCapability{
		AcceptsOps: []Operation{OpInsert},
		Atomicity:  AtomicityBatch,
	}
	checkpoint := CheckpointNone

	switch t.OrDefault() {
	case IngestionSnapshotReplace:
		capability.Mode = WriteReplace
		capability.Atomicity = AtomicityResource
	case IngestionSnapshotUpsert:
		capability.Mode = WriteUpsert
		capability.RequiresPK = true
		checkpoint = CheckpointAfterBatch
	case IngestionIncrementalUpsert:
		capability.Mode = WriteUpsert
		capability.RequiresPK = true
		capability.AcceptsOps = []Operation{OpInsert, OpUpdate}
		checkpoint = CheckpointAfterBatch
	case IngestionSnapshotAppend:
		capability.Mode = WriteAppend
	case IngestionIncrementalAppend:
		capability.Mode = WriteAppend
		capability.AcceptsOps = []Operation{OpInsert, OpUpdate}
		checkpoint = CheckpointAfterBatch
	case IngestionIncrementalDelete:
		capability.Mode = WriteDelete
		capability.RequiresPK = true
		capability.AcceptsOps = []Operation{OpDelete}
		checkpoint = CheckpointAfterBatch
	case IngestionCDC:
		capability.Mode = WriteMerge
		capability.RequiresPK = true
		capability.RequiresOrder = true
		capability.AcceptsOps = []Operation{OpInsert, OpUpdate, OpDelete}
		checkpoint = CheckpointAfterCommit
	}

	return WritePolicy{Capability: capability, Checkpoint: checkpoint}
}

// SourcePolicyForIngestion derives the canonical read-side policy for an
// ingestion type.
func SourcePolicyForIngestion(t IngestionType) SourcePolicy {
	switch t.OrDefault() {
	case IngestionCDC:
		return SourcePolicy{
			Mode:          ModeCDC,
			EmitsOps:      []Operation{OpInsert, OpUpdate, OpDelete},
			Ordered:       true,
			Checkpointing: CheckpointAfterCommit,
		}
	case IngestionSnapshotUpsert:
		return SourcePolicy{
			Mode:          ModeFull,
			EmitsOps:      []Operation{OpInsert},
			Checkpointing: CheckpointAfterBatch,
		}
	case IngestionIncrementalAppend, IngestionIncrementalUpsert, IngestionIncrementalDelete:
		return SourcePolicy{
			Mode:          ModeIncremental,
			EmitsOps:      []Operation{OpInsert, OpUpdate},
			Ordered:       true,
			Checkpointing: CheckpointAfterBatch,
		}
	default:
		return SourcePolicy{
			Mode:          ModeFull,
			EmitsOps:      []Operation{OpInsert},
			Checkpointing: CheckpointNone,
		}
	}
}
