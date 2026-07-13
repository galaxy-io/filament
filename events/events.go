// Package events defines filament's typed event catalog: every fact the
// system emits, its payload type, and the machinery facts travel through —
// subjects (subject.go), wire framing (wire.go), emit paths (emit.go), and
// typed consumption (mux.go).
package events

import (
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
)

// Envelope is the routing identity every fact shares: who and what it is
// about, plus its producer-assigned sequence and emit time.
type Envelope struct {
	Tenant   ingestion.TenantID
	Run      ingestion.RunID
	Resource string
	Seq      uint64
	At       time.Time
}

// Event is the typed view of one delivered fact.
type Event[T any] struct {
	Envelope
	Data T
}

// Fact is the untyped form of one fact — what actually travels the bus. Data
// holds the payload type defined under Name. Consumers narrow it back through
// Mux or On; Event[T] is its typed view.
type Fact struct {
	Envelope
	Name string
	Data any
}

// EventType identifies one defined event kind. The value is the capability to
// emit or subscribe to that kind; construct it only through Define.
type EventType[T any] struct {
	entity string
	name   string
}

// Name returns the wire spelling "<entity>.<event>" (e.g. "batch.written").
func (t EventType[T]) Name() string { return t.entity + eventbus.Separator + t.name }

// IsValid returns boolean if event to be emitted is not empty or malformed
func (t EventType[T]) IsValid() bool {
	if t.entity == "" || t.name == "" {
		return false
	}

	return true
}

// NewFact builds a well-formed Fact for t — the shape producers hand to an
// emit callback when publishing happens elsewhere (see Publish).
func NewFact[T any](t EventType[T], env Envelope, data T) Fact {
	return Fact{Envelope: env, Name: t.Name(), Data: data}
}
