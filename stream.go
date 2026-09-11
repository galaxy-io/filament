package filament

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament/rowmodel"
)

type ExecutionMode string

const (
	ExecutionBounded    ExecutionMode = "bounded"
	ExecutionContinuous ExecutionMode = "continuous"
)

func (e ExecutionMode) Normalize() ExecutionMode {
	if e == "" {
		return ExecutionBounded
	}
	return e
}

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

type InputSemantics string
type Ordering string
type DeliveryGuarantee string

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

type StreamCapabilities struct {
	Input    InputSemantics
	Ordering []Ordering
	Delivery DeliveryGuarantee
}
type Boundary struct {
	MaxRecords      int
	MaxBytes        int64
	MaxWait, MaxAge time.Duration
}
type StreamMeta = rowmodel.StreamMeta
type EventIdentity = rowmodel.EventIdentity
type DomainKey = rowmodel.DomainKey
type Position = rowmodel.Position
type DomainPosition = rowmodel.DomainPosition
type DomainPositions = rowmodel.DomainPositions
type PositionOrder = rowmodel.PositionOrder
type PositionCodec = rowmodel.PositionCodec
type CodecResolver = rowmodel.CodecResolver
type Control = rowmodel.Control
type ControlKind = rowmodel.ControlKind
type ResourceDomainRef = rowmodel.ResourceDomainRef

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

type StreamSource interface {
	OpenStream(context.Context, StreamOpenOpts) (StreamSession, error)
}
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
