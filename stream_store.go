package filament

import (
	"context"
	"time"
)

// StreamRuntimeStore is an optional engine extension, not part of DataStore or
// connector lifecycles. Definitions only: no backend implements it yet.
// All operations are tenant-scoped. StartAttempt, renewal and certification must
// serialize authority. Tokens increase across generations. Identical certificate
// retries return historical success without granting current ack authority.
// CommitEpoch must verify sealed source/barrier evidence in addition to these
// public value types; the transaction and evidence contract lands in PRs 04–06.
type StreamRuntimeStore interface {
	StartAttempt(context.Context, StartAttemptRequest) (Attempt, error)
	RenewLease(context.Context, LeaseToken, time.Duration) error
	EndAttempt(context.Context, EndAttemptRequest) error
	LoadStreamState(context.Context, StreamStateRequest) (StreamState, error)
	ListReconcileCandidates(context.Context, ReconcileQuery) (ReconcilePage, error)
	SetDesiredState(context.Context, DesiredStateChange) error
	CommitEpoch(context.Context, EpochCertificate) (CommittedEpoch, error)
	GetEpoch(context.Context, EpochLookup) (CommittedEpoch, error)
}

// StreamDesiredState records intended execution independently of worker health.
type StreamDesiredState string

// Desired stream states guide whether a supervisor should admit further attempts.
const (
	StreamEnabled StreamDesiredState = "enabled"
	StreamPaused  StreamDesiredState = "paused"
	StreamStopped StreamDesiredState = "stopped"
)

// StreamStateRequest identifies a tenant-scoped stream generation.
type StreamStateRequest struct {
	Tenant TenantID
	Stream StreamRef
}

// LeaseToken binds complete attempt ownership to its tenant.
type LeaseToken struct {
	Tenant  TenantID
	Attempt AttemptRef
}

// StartAttemptRequest supplies immutable execution identity and expected state for admission.
type StartAttemptRequest struct {
	Tenant                                TenantID
	Stream                                StreamRef
	Run                                   RunID
	ExecutionID, PipelineVersionID, Route string
	ExpectedRevision                      int64
	TTL                                   time.Duration
}

// Attempt describes one admitted worker lifetime and its lease interval.
type Attempt struct {
	Lease                LeaseToken
	StartedAt, ExpiresAt time.Time
	EndedAt              *time.Time
}

// AttemptTermination describes how a worker ended.
// Successful session closure, not inferred workload disappearance, is necessary
// to record clean termination. Detailed takeover proofs land with their owner.
type AttemptTermination string

// Termination outcomes distinguish proven closure from inferred or unproven shutdown.
const (
	AttemptClean    AttemptTermination = "clean"
	AttemptReaped   AttemptTermination = "reaped"
	AttemptUnproven AttemptTermination = "unproven"
)

// EndAttemptRequest conditionally records termination under the owning lease.
type EndAttemptRequest struct {
	Lease       LeaseToken
	Termination AttemptTermination
	Reason      string
}

// StreamState provides authoritative intent and recovery state for a stream generation.
type StreamState struct {
	StreamStateRequest
	Desired                  StreamDesiredState
	Revision                 int64
	PipelineVersionID, Route string
	Run                      RunID
	Attempt                  *Attempt
	CommittedPositions       DomainPositions
	LastEpoch                *EpochKey
	MembershipRevision       int64
	Membership               map[string]ReplicationStreamResourceStatus
}

// ReconcileQuery selects a bounded page of tenant-scoped reconciliation candidates.
type ReconcileQuery struct {
	Tenant TenantID
	After  string
	Limit  int
}

// ReconcilePage returns stream states and an opaque continuation cursor.
type ReconcilePage struct {
	States []StreamState
	Next   string
}

// DesiredStateChange requests an intent update guarded by the expected revision.
type DesiredStateChange struct {
	StreamStateRequest
	ExpectedRevision int64
	Desired          StreamDesiredState
}

// EpochLookup identifies a historical certificate within its tenant scope.
type EpochLookup struct {
	Tenant TenantID
	Key    EpochKey
}
