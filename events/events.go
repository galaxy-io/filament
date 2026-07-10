package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
)

type Envelope struct {
	Tenant   ingestion.TenantID
	Run      ingestion.RunID
	Resource string
	Seq      uint64
	At       time.Time
}

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

// Definition is one registry entry: the subject coordinates and payload
// decoder of a defined event kind.
type Definition struct {
	Entity string
	Event  string
	decode func([]byte) (any, error)
}

// Name returns the wire spelling "<entity>.<event>".
func (d Definition) Name() string { return d.Entity + eventbus.Separator + d.Event }

var (
	regMu    sync.RWMutex
	registry = map[string]Definition{}
)

// define registers an event kind under its wire spelling "<entity>.<event>"
// (e.g. "batch.written") and returns its EventType. A malformed or duplicate
// name panics: the catalog is a compile-time artifact and collisions are bugs.
func define[T any](name string) EventType[T] {
	entity, event, ok := strings.Cut(name, eventbus.Separator)
	if !ok || entity == "" || event == "" || strings.Contains(event, eventbus.Separator) {
		panic(fmt.Sprintf("events: %q is not <entity>.<event>", name))
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("events: %q defined twice", name))
	}
	registry[name] = Definition{
		Entity: entity,
		Event:  event,
		decode: func(b []byte) (any, error) {
			var v T
			if len(b) > 0 {
				if err := json.Unmarshal(b, &v); err != nil {
					return nil, err
				}
			}
			return v, nil
		},
	}
	return EventType[T]{entity: entity, name: event}
}

// Lookup returns the Definition registered under the wire spelling.
func Lookup(name string) (Definition, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	d, ok := registry[name]
	return d, ok
}

// Names returns every registered wire spelling, sorted.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// wireEvent is the self-describing JSON frame a Fact crosses transports as.
type wireEvent struct {
	Type     string          `json:"type"`
	Tenant   string          `json:"tenant"`
	Run      string          `json:"run"`
	Resource string          `json:"resource,omitempty"`
	Seq      uint64          `json:"seq"`
	At       time.Time       `json:"at"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// Marshal frames one Fact for the wire.
func Marshal(f Fact) ([]byte, error) {
	raw, err := json.Marshal(f.Data)
	if err != nil {
		return nil, fmt.Errorf("events: marshal %s payload: %w", f.Name, err)
	}
	return json.Marshal(wireEvent{
		Type:     f.Name,
		Tenant:   string(f.Tenant),
		Run:      string(f.Run),
		Resource: f.Resource,
		Seq:      f.Seq,
		At:       f.At,
		Data:     raw,
	})
}

// Unmarshal decodes one wire frame back into a Fact, its payload typed by the
// registry. A malformed frame or unregistered type is an error.
func Unmarshal(b []byte) (Fact, error) {
	var w wireEvent
	if err := json.Unmarshal(b, &w); err != nil {
		return Fact{}, fmt.Errorf("events: unmarshal frame: %w", err)
	}
	def, ok := Lookup(w.Type)
	if !ok {
		return Fact{}, fmt.Errorf("events: unknown event type %q", w.Type)
	}
	data, err := def.decode(w.Data)
	if err != nil {
		return Fact{}, fmt.Errorf("events: decode %s payload: %w", w.Type, err)
	}
	return Fact{
		Envelope: Envelope{
			Tenant:   ingestion.TenantID(w.Tenant),
			Run:      ingestion.RunID(w.Run),
			Resource: w.Resource,
			Seq:      w.Seq,
			At:       w.At,
		},
		Name: w.Type,
		Data: data,
	}, nil
}

// Codec frames Facts for transport buses (see eventbus.Codec); the in-process
// bus never uses it — Facts pass by reference. Transports terminate messages
// this codec cannot decode, so poison frames never block a consumer.
var Codec eventbus.Codec = codec{}

type codec struct{}

func (codec) Encode(payload any) ([]byte, error) {
	f, ok := payload.(Fact)
	if !ok {
		return nil, fmt.Errorf("events: codec expected Fact, got %T", payload)
	}
	return Marshal(f)
}

func (codec) Decode(data []byte) (any, error) {
	f, err := Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Decode extracts the Fact from a delivered message — the by-reference value
// in-process, or the codec-decoded one on a transport.
func Decode(msg eventbus.Message) (Fact, error) {
	f, ok := msg.Payload().(Fact)
	if !ok {
		return Fact{}, fmt.Errorf("events: unexpected payload type %T", msg.Payload())
	}
	return f, nil
}

// Subject builds the concrete publish subject for one fact:
// ingestion.v1.<entity>.<tenant>.<run>.<event>.
func Subject[T any](t EventType[T], tenant ingestion.TenantID, run ingestion.RunID) string {
	return strings.Join([]string{
		ingestion.SubjectPrefix, ingestion.SubjectVersion,
		t.entity, string(tenant), string(run), t.name,
	}, eventbus.Separator)
}

// SubjectPattern is the subscription pattern matching every fact of one kind:
// ingestion.v1.<entity>.*.*.<event>.
func SubjectPattern[T any](t EventType[T]) string {
	return strings.Join([]string{
		ingestion.SubjectPrefix, ingestion.SubjectVersion,
		t.entity, eventbus.TokenWildcard, eventbus.TokenWildcard, t.name,
	}, eventbus.Separator)
}

// Emit validates the envelope and publishes the fact under its subject. The
// payload travels as a Fact value; framing happens only at a transport edge.
func Emit[T any](ctx context.Context, bus eventbus.Bus, t EventType[T], env Envelope, data T) error {
	if !t.IsValid() {
		return errors.New("events: emit: invalid EventType")
	}
	if err := env.Tenant.Valid(); err != nil {
		return fmt.Errorf("events: emit %s: tenant: %w", t.Name(), err)
	}
	if err := env.Run.Valid(); err != nil {
		return fmt.Errorf("events: emit %s: run: %w", t.Name(), err)
	}
	f := Fact{Envelope: env, Name: t.Name(), Data: data}
	return bus.Publish(ctx, Subject(t, env.Tenant, env.Run), f)
}

// On subscribes to one event kind on its own subscription and pumps typed
// deliveries to fn. nil ⇒ ack; non-nil ⇒ nak (redeliver). Canceling ctx closes
// the subscription, as does the returned func. Modules consuming many kinds
// through one durable consumer use Mux instead.
func On[T any](ctx context.Context, bus eventbus.Bus, t EventType[T], opts eventbus.SubOpts, fn func(context.Context, Event[T]) error) (func(), error) {
	if !t.IsValid() {
		return nil, errors.New("events: on: invalid EventType")
	}
	sub, err := bus.Subscribe(SubjectPattern(t), opts)
	if err != nil {
		return nil, err
	}

	// Ends the subscription at the bus level on ctx cancel; done releases the
	// watcher when the pump exits first.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Close()
		case <-done:
		}
	}()

	go func() {
		defer close(done)
		for msg := range sub.C() {
			if ctx.Err() != nil {
				_ = msg.Nak() // canceled mid-flight: redeliver, don't execute
				continue
			}
			f, err := Decode(msg)
			if err != nil {
				_ = msg.Ack() // not a Fact: a foreign publisher on our pattern — skip
				continue
			}
			payload, ok := f.Data.(T)
			if !ok {
				_ = msg.Ack()
				continue
			}
			if err := fn(ctx, Event[T]{Envelope: f.Envelope, Data: payload}); err != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}()
	return func() { _ = sub.Close() }, nil
}

// Mux fans one subscription out to typed handlers by event kind — the shape
// for modules that consume many kinds through a single durable consumer
// (one cursor, one dedup sequence). Wire Dispatch as the subscription handler.
type Mux struct {
	mu       sync.RWMutex
	handlers map[string][]func(context.Context, Envelope, any) error
	anys     []func(context.Context, Fact) error
}

// NewMux returns an empty Mux.
func NewMux() *Mux {
	return &Mux{handlers: map[string][]func(context.Context, Envelope, any) error{}}
}

// Handle attaches a typed handler for one event kind. (A free function: Go
// methods cannot take type parameters.)
func Handle[T any](m *Mux, t EventType[T], fn func(context.Context, Event[T]) error) {
	wrapped := func(ctx context.Context, env Envelope, data any) error {
		payload, ok := data.(T)
		if !ok {
			return fmt.Errorf("events: %s payload is %T", t.Name(), data)
		}
		return fn(ctx, Event[T]{Envelope: env, Data: payload})
	}
	m.mu.Lock()
	m.handlers[t.Name()] = append(m.handlers[t.Name()], wrapped)
	m.mu.Unlock()
}

// HandleAny attaches a handler for every event kind — for forwarding
// consumers that treat facts uniformly.
func (m *Mux) HandleAny(fn func(context.Context, Fact) error) {
	m.mu.Lock()
	m.anys = append(m.anys, fn)
	m.mu.Unlock()
}

// Dispatch extracts one delivered Fact and runs every matching handler. All
// handlers run even when an earlier one fails; errors are joined so the
// caller naks once. A payload that is not a Fact (a foreign publisher on the
// consumed pattern) is skipped — redelivery cannot fix it.
func (m *Mux) Dispatch(ctx context.Context, msg eventbus.Message) error {
	f, err := Decode(msg)
	if err != nil {
		return nil
	}

	m.mu.RLock()
	typed := append([]func(context.Context, Envelope, any) error(nil), m.handlers[f.Name]...)
	anys := append([]func(context.Context, Fact) error(nil), m.anys...)
	m.mu.RUnlock()

	var errs []error
	for _, fn := range typed {
		if err := fn(ctx, f.Envelope, f.Data); err != nil {
			errs = append(errs, err)
		}
	}
	for _, fn := range anys {
		if err := fn(ctx, f); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
