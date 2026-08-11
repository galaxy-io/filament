package filament

import (
	"fmt"
	"slices"
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
	// IngestionTypes maps each resource to its ingestion type; the "" entry is
	// the route default for resources not explicitly listed.
	IngestionTypes map[string]IngestionType
	Checkpoint     *CheckpointData
	Options        RunOptions
	WritePolicies  map[string]WritePolicy
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
	// IngestionTypes maps each resource to its ingestion type; the "" entry is
	// the route default for resources not explicitly listed.
	IngestionTypes  map[string]IngestionType
	CheckpointRoute string
	CursorConfigs   map[string]ResourceCursorConfig
	Options         RunOptions
	ScheduleID      ScheduleID
	// ScheduledFor is the occurrence this request represents; zero when manual.
	ScheduledFor time.Time
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

// TypeFor returns the ingestion type governing resource: its per-resource
// entry when one exists, otherwise the route default ("" entry).
func TypeFor(types map[string]IngestionType, resource string) IngestionType {
	if t, ok := types[resource]; ok {
		return t.OrDefault()
	}
	return types[""].OrDefault()
}

// IsCDC reports whether types replicates a change stream. CDC never mixes
// with other types on one route, so any CDC entry means the whole run is CDC.
func IsCDC(types map[string]IngestionType) bool {
	for _, t := range types {
		if t == IngestionCDC {
			return true
		}
	}
	return false
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
	Run       RunID
	Tenant    TenantID
	Status    RunStatus
	Request   RunRequest
	Resources []ResourceState
	Records   int64
	Bytes     int64

	// Lifecycle stamps, first-write-wins in the store. Created on insert,
	// scheduled at the occurrence's fire time, requested when run.requested is
	// emitted, started and finished folded from the run's own facts. Zero is
	// unset.
	CreatedAt   time.Time
	ScheduledAt time.Time
	RequestedAt time.Time
	StartedAt   time.Time
	FinishedAt  *time.Time
	UpdatedAt   time.Time

	Error      string
	ScheduleID ScheduleID
	// Folded from run.heartbeat facts: cumulative worker CPU time and the
	// peak working set observed over the run.
	CPUSeconds      float64
	MemoryPeakBytes int64
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
	// RunScheduled is a run pre-created for a schedule's next occurrence, before
	// its fire time. It precedes RunRequested in lifecycle order but is declared
	// last so persisted ordinals stay stable and the zero value stays RunRequested.
	// Owned entirely by the control plane: the scheduler creates it and promotes
	// it to RunRequested at fire; nothing downstream ever sees it.
	RunScheduled
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
// Since is inclusive and Until exclusive on StartedAt — a window asks which
// runs ran in it, so runs that never started fall outside either bound.
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
	IngestionFullReplace       IngestionType = "full_replace"
	IngestionFullUpsert        IngestionType = "full_upsert"
	IngestionFullAppend        IngestionType = "full_append"
	IngestionIncrementalAppend IngestionType = "incremental_append"
	IngestionIncrementalUpsert IngestionType = "incremental_upsert"
	IngestionIncrementalDelete IngestionType = "incremental_delete"
	IngestionCDC               IngestionType = "cdc"
)

// OrDefault substitutes IngestionFullReplace for the empty type.
func (t IngestionType) OrDefault() IngestionType {
	if t == "" {
		return IngestionFullReplace
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
	WriteAppend  WriteMode = "append"
	WriteReplace WriteMode = "replace"
	WriteUpsert  WriteMode = "upsert"
	WriteDelete  WriteMode = "delete"
	WriteMerge   WriteMode = "merge"
)

// IngestionFor compiles the two user levers — per-table read mode and sink
// write mode — into the internal ingestion type. Zero levers default to a
// full-refresh replace; an unset write on an incremental read defaults to
// upsert. CDC connections never reach this: their edges are always
// IngestionCDC.
//
// Delete and merge are engine mechanisms rather than levers: they are derived
// from the source's operations, never chosen. IngestionFor still compiles
// delete so internal callers can name the type; WriteModesFor is what decides
// what a user may pick.
func IngestionFor(read ReadMode, write WriteMode) (IngestionType, error) {
	if write == "" {
		if read == ModeIncremental {
			write = WriteUpsert
		} else {
			write = WriteReplace
		}
	}
	switch read {
	case ModeFull:
		switch write {
		case WriteReplace:
			return IngestionFullReplace, nil
		case WriteUpsert:
			return IngestionFullUpsert, nil
		case WriteAppend:
			return IngestionFullAppend, nil
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
	return "", fmt.Errorf("read mode %q cannot combine with write mode %q", read, write)
}

// WriteModesFor returns the write modes a user may pair with read, in menu
// order — the inverse of IngestionFor, and the one place that decides what a
// write lever offers. CDC reads have no lever and yield none.
func WriteModesFor(read ReadMode) []WriteMode {
	switch read {
	case ModeFull:
		return []WriteMode{WriteAppend, WriteReplace, WriteUpsert}
	case ModeIncremental:
		return []WriteMode{WriteAppend, WriteUpsert}
	}
	return nil
}

// String renders a read mode for messages and logs.
func (m ReadMode) String() string {
	switch m {
	case ModeIncremental:
		return "incremental"
	case ModeCDC:
		return "cdc"
	default:
		return "full"
	}
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

// VersionStrategy selects the ordering value an insert-based upsert sink uses
// when several records share the same primary key.
type VersionStrategy string

const (
	// VersionInsertOrder leaves conflict ordering to the sink's insertion and
	// merge semantics. It is the fallback when no row-level cursor is available.
	VersionInsertOrder VersionStrategy = "insert_order"
	// VersionCursor orders rows by the source field used for incremental reads.
	// The field must be present, non-null, and advance on every source update.
	VersionCursor VersionStrategy = "cursor"
)

// VersionPolicy tells a sink how to resolve competing values for one primary
// key. Field and its types are populated for VersionCursor; they are empty for
// VersionInsertOrder.
type VersionPolicy struct {
	Strategy VersionStrategy
	Field    string
	Logical  LogicalType
	Native   string
}

// Eventually we might want to track a deleted-at tombstone alongside the
// version cursor, but the engine and API do not support tombstones yet.
//
// type TombstonePolicy struct {
// 	Field string // e.g. deleted_at; non-null means deleted
// }

// WritePolicy binds a capability to one resource's keys and checkpoint timing
// — the per-resource contract handed to a sink via ApplyOptions.
type WritePolicy struct {
	Capability WritePolicyCapability
	Resource   string
	Keys       []string
	Version    VersionPolicy
	Checkpoint CheckpointPolicy
}

// Accepts reports whether the capability admits op; an empty AcceptsOps
// admits everything.
func (c WritePolicyCapability) Accepts(op Operation) bool {
	if len(c.AcceptsOps) == 0 {
		return true
	}
	return slices.Contains(c.AcceptsOps, op)
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
	Mode          ReadMode
	EmitsOps      []Operation
	Ordered       bool
	Checkpointing CheckpointPolicy
}

// IngestionPlan is the resolved policy set for a run: per-resource write
// policies, each bound from its resource's own ingestion type.
type IngestionPlan struct {
	WritePolicies map[string]WritePolicy
	RequiresCDC   bool
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
	case IngestionFullReplace:
		capability.Mode = WriteReplace
		capability.Atomicity = AtomicityResource
	case IngestionFullUpsert:
		capability.Mode = WriteUpsert
		capability.RequiresPK = true
		checkpoint = CheckpointAfterBatch
	case IngestionIncrementalUpsert:
		capability.Mode = WriteUpsert
		capability.RequiresPK = true
		capability.AcceptsOps = []Operation{OpInsert, OpUpdate}
		checkpoint = CheckpointAfterBatch
	case IngestionFullAppend:
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

	policy := WritePolicy{Capability: capability, Checkpoint: checkpoint}
	if capability.Mode == WriteUpsert {
		policy.Version.Strategy = VersionInsertOrder
	}
	return policy
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
	case IngestionFullUpsert:
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
