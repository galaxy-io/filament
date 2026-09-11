// Package eventbus is a transport-only, payload-agnostic event bus shared
// across the system. It routes opaque payloads to subscribers by string
// subject using NATS-style wildcard patterns, knowing nothing of any domain
// vocabulary.
//
// Each domain layers its own typed event, subject grammar, and codec on top.
// Payloads travel as any: the in-process bus passes them by reference with zero
// marshaling; transports that cross a process boundary marshal at the edge via
// a domain-supplied [Codec].
package eventbus

import (
	"context"
	"time"
)

// Bus delivers payloads published under a subject to every subscription whose
// pattern matches. Implementations must be safe for concurrent use.
type Bus interface {
	// Publish routes payload to matching subscriptions. subject must be
	// concrete (wildcard-free); the payload is never inspected.
	Publish(ctx context.Context, subject string, payload any) error

	// Subscribe registers interest in a pattern ('*' = one token, trailing
	// '>' = one or more).
	Subscribe(pattern string, opts SubOpts) (Subscription, error)

	// Resolver resolves a pattern to this transport's Route. Subscribe uses it;
	// every provider declares its own.
	Resolver

	// Name identifies the transport for logs (e.g. "inproc", "nats").
	Name() string
}

// ReadinessChecker is an optional transport capability used by serving
// processes to stop accepting work when their event bus is unavailable.
type ReadinessChecker interface {
	Ready(context.Context) error
}

// Subscription is a live stream of matching messages. Close unsubscribes and
// closes C.
type Subscription interface {
	C() <-chan Message
	Close() error
}

// Message is one delivered payload and its routing metadata. Payload is the
// value as published (the typed event on an in-process bus, the codec-decoded
// form on a transport).
type Message interface {
	Subject() string
	Payload() any
	Seq() uint64
	Ack() error
	Nak() error
}

// ProgressReporter is an optional message capability that extends the acknowledgement window.
// Call InProgress periodically while handling a long-running message.
type ProgressReporter interface {
	InProgress() error
}

// Replayable is an optional capability: a subscription that receives the
// retained backlog from fromSeq before tailing live.
type Replayable interface {
	Replay(ctx context.Context, pattern string, fromSeq uint64) (Subscription, error)
}

// SubOpts tunes a subscription. The zero value is an ephemeral live tail.
type SubOpts struct {
	Durable     string        // "" = ephemeral (live tail)
	FromSeq     uint64        // replay start (with Replayable)
	AckWait     time.Duration // redelivery window before retry
	MaxInFlight int           // max unacked messages (0 = transport default)

	// Replay delivers the full retained backlog when the subscription is first
	// created instead of only new messages. Only safe for consumers that fold
	// facts idempotently (dedup on Seq); command consumers that execute work
	// must not set it.
	Replay bool
}
