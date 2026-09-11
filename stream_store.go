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
type StreamDesiredState string

const (
	StreamEnabled StreamDesiredState = "enabled"
	StreamPaused  StreamDesiredState = "paused"
	StreamStopped StreamDesiredState = "stopped"
)

type StreamStateRequest struct {
	Tenant TenantID
	Stream StreamRef
}
type LeaseToken struct {
	Tenant  TenantID
	Attempt AttemptRef
}
type StartAttemptRequest struct {
	Tenant                                TenantID
	Stream                                StreamRef
	Run                                   RunID
	ExecutionID, PipelineVersionID, Route string
	ExpectedRevision                      int64
	TTL                                   time.Duration
}
type Attempt struct {
	Lease                LeaseToken
	StartedAt, ExpiresAt time.Time
	EndedAt              *time.Time
}

// Successful session closure, not inferred workload disappearance, is necessary
// to record clean termination. Detailed takeover proofs land with their owner.
type AttemptTermination string

const (
	AttemptClean    AttemptTermination = "clean"
	AttemptReaped   AttemptTermination = "reaped"
	AttemptUnproven AttemptTermination = "unproven"
)

type EndAttemptRequest struct {
	Lease       LeaseToken
	Termination AttemptTermination
	Reason      string
}
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
type ReconcileQuery struct {
	Tenant TenantID
	After  string
	Limit  int
}
type ReconcilePage struct {
	States []StreamState
	Next   string
}
type DesiredStateChange struct {
	StreamStateRequest
	ExpectedRevision int64
	Desired          StreamDesiredState
}
type EpochLookup struct {
	Tenant TenantID
	Key    EpochKey
}
