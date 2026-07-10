package ingestion

import (
	"fmt"
	"time"
)

type RunSpec struct {
	Tenant        TenantID
	Run           RunID
	Source        Ref
	Sink          Ref
	DataStore     Ref
	Resources     []string
	Selectors     []string
	IngestionType IngestionType
	Mode          ReplicationMode
	Checkpoint    *CheckpointData
	Options       RunOptions
}

type RunRequest struct {
	Tenant         TenantID
	IdempotencyKey string
	Source         Ref
	Sink           Ref
	DataStore      Ref
	Resources      []string
	Selectors      []string
	IngestionType  IngestionType
	Options        RunOptions
}

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

type RunStatus int

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

type RunResult struct {
	Status  RunStatus
	Records int64
	Bytes   int64
	URIs    []string
	Error   string
}

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

type RunFilter struct {
	Tenant   TenantID
	Source   string
	Status   []RunStatus
	Schedule ScheduleID
	Since    time.Time
	Limit    int
	Cursor   string
}

type SyncSnapshot struct {
	Run       RunState
	Resources []ResourceState
	AtSeq     uint64
}

type IngestionType string

const (
	IngestionSnapshotReplace IngestionType = "snapshot_replace"
	IngestionSnapshotUpsert  IngestionType = "snapshot_upsert"
	IngestionAppend          IngestionType = "append"
	IngestionUpsert          IngestionType = "upsert"
	IngestionDelete          IngestionType = "delete"
	IngestionCDC             IngestionType = "cdc"
)

func (t IngestionType) OrDefault() IngestionType {
	if t == "" {
		return IngestionSnapshotReplace
	}
	return t
}

func (t IngestionType) WritePolicy() WritePolicy {
	return WritePolicyForIngestion(t)
}

func (t IngestionType) WriteCapability() WritePolicyCapability {
	return t.WritePolicy().Capability
}

func (t IngestionType) SourcePolicy() SourcePolicy {
	return SourcePolicyForIngestion(t)
}

func WriteCapabilities(types ...IngestionType) []WritePolicyCapability {
	out := make([]WritePolicyCapability, 0, len(types))
	for _, t := range types {
		out = append(out, t.WriteCapability())
	}
	return out
}

func SourcePolicies(types ...IngestionType) []SourcePolicy {
	out := make([]SourcePolicy, 0, len(types))
	for _, t := range types {
		out = append(out, t.SourcePolicy())
	}
	return out
}

type WriteMode string

const (
	WriteAppend  WriteMode = "append"
	WriteReplace WriteMode = "replace"
	WriteUpsert  WriteMode = "upsert"
	WriteDelete  WriteMode = "delete"
	WriteMerge   WriteMode = "merge"
)

type WriteAtomicity string

const (
	AtomicityBatch    WriteAtomicity = "batch"
	AtomicityResource WriteAtomicity = "resource"
	AtomicityRun      WriteAtomicity = "run"
)

type CheckpointPolicy string

const (
	CheckpointNone        CheckpointPolicy = "none"
	CheckpointAfterBatch  CheckpointPolicy = "after_batch"
	CheckpointAfterCommit CheckpointPolicy = "after_commit"
)

type WritePolicyCapability struct {
	Mode          WriteMode
	RequiresPK    bool
	RequiresOrder bool
	AcceptsOps    []Operation
	Atomicity     WriteAtomicity
}

type WritePolicy struct {
	Capability WritePolicyCapability
	Resource   string
	Keys       []string
	Checkpoint CheckpointPolicy
}

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

func (p WritePolicy) ValidateRecords(resource string, records []Record) error {
	for _, rec := range records {
		if !p.Capability.Accepts(rec.Op) {
			return fmt.Errorf("write policy %q does not accept %s record for resource %q", p.Capability.Mode, OperationName(rec.Op), resource)
		}
	}
	return nil
}

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

type SourcePolicy struct {
	Mode          ReplicationMode
	EmitsOps      []Operation
	Ordered       bool
	Checkpointing CheckpointPolicy
}

type IngestionPlan struct {
	Type          IngestionType
	SourcePolicy  SourcePolicy
	WritePolicies map[string]WritePolicy
	RequiresCDC   bool
	RequiresPK    bool
}

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
	case IngestionUpsert:
		capability.Mode = WriteUpsert
		capability.RequiresPK = true
		capability.AcceptsOps = []Operation{OpInsert, OpUpdate}
		checkpoint = CheckpointAfterBatch
	case IngestionAppend:
		capability.Mode = WriteAppend
	case IngestionDelete:
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
	case IngestionUpsert, IngestionDelete:
		return SourcePolicy{
			Mode:          ModeIncremental,
			EmitsOps:      []Operation{OpInsert, OpUpdate},
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
