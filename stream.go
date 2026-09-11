package filament

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament/rowmodel"
)

// ExecutionMode selects bounded or continuous execution independently of input semantics.
type ExecutionMode string

// Execution modes retain bounded behavior as the default.
const (
	ExecutionBounded    ExecutionMode = "bounded"
	ExecutionContinuous ExecutionMode = "continuous"
)

// Normalize maps the compatibility zero value to bounded execution.
func (e ExecutionMode) Normalize() ExecutionMode {
	if e == "" {
		return ExecutionBounded
	}
	return e
}

// ErrContinuousDisabled reports that the continuous runtime is unavailable.
var ErrContinuousDisabled = errors.New("continuous execution is disabled")

// Validate checks the mode, independently of runtime availability.
func (e ExecutionMode) Validate() error {
	switch e.Normalize() {
	case ExecutionBounded, ExecutionContinuous:
		return nil
	default:
		return fmt.Errorf("unknown execution mode %q", e)
	}
}

type (
	// InputSemantics describes whether input consists of rows, changes, or messages.
	InputSemantics string
	// Ordering identifies a source guarantee or destination ordering requirement.
	Ordering string
	// DeliveryGuarantee describes input replay and loss semantics.
	DeliveryGuarantee string
)

// Source input, ordering, and delivery capabilities are negotiated independently.
const (
	InputRows                     InputSemantics    = "rows"
	InputChanges                  InputSemantics    = "changes"
	InputMessages                 InputSemantics    = "messages"
	OrderingNone                  Ordering          = "none"
	OrderingPartition             Ordering          = "partition"
	OrderingTransaction           Ordering          = "transaction"
	OrderingGlobalStrict          Ordering          = "global_strict"
	DeliveryReplayableAtLeastOnce DeliveryGuarantee = "replayable_at_least_once"
	DeliveryBestEffort            DeliveryGuarantee = "best_effort"
)

// StreamCapabilities describes source input, ordering, and replay guarantees.
type StreamCapabilities struct {
	Input    InputSemantics
	Ordering []Ordering
	Delivery DeliveryGuarantee
}

// Boundary defines soft record, byte, idle, and age targets for one epoch.
// Complete source transactions may exceed these targets.
type Boundary struct {
	MaxRecords      int
	MaxBytes        int64
	MaxWait, MaxAge time.Duration
}

type (
	// StreamMeta carries optional native event metadata alongside a row.
	StreamMeta = rowmodel.StreamMeta
	// EventIdentity identifies a source event independently of ingestion attempts.
	EventIdentity = rowmodel.EventIdentity
	// DomainKey identifies an ordering domain within a source incarnation.
	DomainKey = rowmodel.DomainKey
	// Position stores codec-encoded source progress.
	Position = rowmodel.Position
	// DomainPosition pairs a domain with its position for serialization.
	DomainPosition = rowmodel.DomainPosition
	// DomainPositions maps source ordering domains to their bookmarks.
	DomainPositions = rowmodel.DomainPositions
	// PositionOrder reports codec-defined ordering between compatible positions.
	PositionOrder = rowmodel.PositionOrder
	// PositionCodec defines validation, comparison, and canonicalization for progress.
	PositionCodec = rowmodel.PositionCodec
	// CodecResolver finds a position codec by name and version.
	CodecResolver = rowmodel.CodecResolver
	// Control carries standalone source boundary evidence.
	Control = rowmodel.Control
	// ControlKind distinguishes transaction, progress, and membership markers.
	ControlKind = rowmodel.ControlKind
	// ResourceDomainRef relates a resource to a shared source ordering domain.
	ResourceDomainRef = rowmodel.ResourceDomainRef
)

// Neutral comparison outcomes and control kinds retain their rowmodel meanings.
const (
	PositionIncomparable = rowmodel.PositionIncomparable
	PositionEqual        = rowmodel.PositionEqual
	PositionBefore       = rowmodel.PositionBefore
	PositionAfter        = rowmodel.PositionAfter
	ControlUnspecified   = rowmodel.ControlUnspecified
	TxnBegin             = rowmodel.TxnBegin
	TxnEnd               = rowmodel.TxnEnd
	ProgressBoundary     = rowmodel.ProgressBoundary
	MemberActivated      = rowmodel.MemberActivated
)

// StreamSource is an optional native factory for reusable source sessions.
type StreamSource interface {
	OpenStream(context.Context, StreamOpenOpts) (StreamSession, error)
}

// StreamOpenOpts supplies selected resources, durable resume state, and admitted ownership.
type StreamOpenOpts struct {
	Resources          []string
	CommittedPositions DomainPositions
	Membership         map[string]ReplicationStreamResourceStatus
	Attempt            AttemptRef
}

// StreamSession owns its connection and protocol liveness under backpressure.
// Read and Acknowledge are serialized. Cancel and join an outstanding call before
// Close; concurrent Close is not required. Cancellation is never a safe boundary.
// Read returns candidate complete-source coverage; only the coordinator can
// certify pipeline completion and durability. Acknowledge receives certified
// coverage and must still check current provider authority.
type StreamSession interface {
	Read(context.Context, StreamRecordSink, Boundary) (Coverage, error)
	Acknowledge(context.Context, Coverage) error
	Close(context.Context) error
}

// StreamRecordSink extends row writing with owner-ordered source boundary controls.
type StreamRecordSink interface {
	RecordSink
	Control(context.Context, Control) error
}

// StreamingSinkCapabilities are claims requiring connector-specific evidence.
// OwnerFencing checks authority across generations atomically with effects.
// IsolatedEpochs additionally requires independent contexts (a later contract).
// InFlightBound is nil when unknown; when set it must be positive and enforced
// server-side over all residual effects after a proven end to submissions.
type StreamingSinkCapabilities struct {
	OwnerFencing, IsolatedEpochs bool
	InFlightBound                *time.Duration
}

// ValidateStreamAttempt checks admitted continuous ownership against duplicated
// run/dispatch identities. It does not grant authority or check runtime support.
// Bounded specifications do not interpret this optional continuous context.
func (s RunSpec) ValidateStreamAttempt() error {
	if err := s.Options.Execution.Validate(); err != nil {
		return err
	}
	if s.Options.Execution.Normalize() != ExecutionContinuous {
		return nil
	}
	if s.StreamAttempt == nil {
		return errors.New("stream: admitted attempt required")
	}
	a := s.StreamAttempt
	if err := a.Validate(); err != nil {
		return err
	}
	if (s.Run != "" && s.Run != a.RunID) || (s.ExecutionID != "" && s.ExecutionID != a.ExecutionID) ||
		(s.ReplicationStream != nil && (s.ReplicationStream.ID != a.StreamID || s.ReplicationStream.Generation != a.Generation)) {
		return errors.New("stream: run identity conflicts with admitted attempt")
	}
	return nil
}
