package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/galaxy-io/filament/eventbus"
)

// Event is an ingestion fact. It travels the eventbus as the payload: passed by
// reference in-process, or codec-encoded across a transport (see JSONCodec).
type Event struct {
	Type     EventType
	Tenant   TenantID
	Run      RunID
	Resource string
	Seq      uint64
	At       time.Time
	Fields   EventFields
}

type EventType int

const (
	EvUnspecified EventType = iota
	EvRunRequested
	EvRunStarted
	EvRunCompleted
	EvRunFailed
	EvHeartbeat
	EvResourceStarted
	EvPageFetched
	EvResourceCompleted
	EvResourceFailed
	EvBatchBuffered
	EvBatchWritten
	EvIntegrityVerified
	EvChunkDivergence
	EvWatermarkAdvanced
	EvCheckpointSaved
	EvRateLimited
	EvRetryExhausted
	EvScheduleFired
	// EvRunPartial is the non-terminal failure of a resumable run: extraction stopped
	// with progress checkpointed. Folds to RunPartial, from which a re-requested run resumes.
	EvRunPartial
)

// EventFields is a payload union; only the fields relevant to Type are set.
type EventFields struct {
	Records    int64
	Bytes      int64
	URI        string
	CRC        uint32
	Checkpoint *CheckpointData
	Error      string
	RetryAfter time.Duration // for EvRateLimited
}

// PublishEvent validates ev and publishes it under its derived subject. It is
// the domain entry point onto the bus, so every published fact is well-formed.
func PublishEvent(ctx context.Context, bus eventbus.Bus, ev Event) error {
	if err := ev.Validate(); err != nil {
		return fmt.Errorf("pkg: publish: %w", err)
	}
	return bus.Publish(ctx, ev.Subject(), ev)
}

// EventOf extracts the typed Event from a delivered message: the pass-by-
// reference payload (in-proc) or codec-decoded bytes (transport).
func EventOf(msg eventbus.Message) (Event, error) {
	switch p := msg.Payload().(type) {
	case Event:
		return p, nil
	case []byte:
		v, err := JSONCodec.Decode(p)
		if err != nil {
			return Event{}, err
		}
		ev, ok := v.(Event)
		if !ok {
			return Event{}, fmt.Errorf("pkg: codec decoded %T, want Event", v)
		}
		return ev, nil
	default:
		return Event{}, fmt.Errorf("pkg: unexpected payload type %T", p)
	}
}

// Subject grammar — the addressing scheme every fact travels under.
//
//	ingestion.v1.<entity>.<tenant>.<run>.<event>
//	e.g. ingestion.v1.run.T123.R789.started
//	     ingestion.v1.resource.T123.R789.page_fetched
//
// Exactly six dot-separated tokens. The resource name is not in the subject
const (
	SubjectPrefix  = "ingestion"
	SubjectVersion = "v1"

	subjectSep    = eventbus.Separator
	subjectTokens = 6 // prefix . version . entity . tenant . run . event

	// Wildcards follow NATS semantics, understood by SubjectMatch.
	TokenWildcard = eventbus.TokenWildcard // matches exactly one token
	TailWildcard  = eventbus.TailWildcard  // matches one or more trailing tokens
)

// Entity is the second-to-... grouping token: which family a fact belongs to.
type Entity string

const (
	EntityRun      Entity = "run"
	EntityResource Entity = "resource"
	EntityBatch    Entity = "batch"
	EntityCursor   Entity = "cursor"
	EntityPressure Entity = "pressure"
	EntitySchedule Entity = "schedule"
	EntityUnknown  Entity = "unknown"
)

// eventMeta maps an EventType to its subject coordinates.
type eventMeta struct {
	entity Entity
	name   string
}

var eventMetaTable = map[EventType]eventMeta{
	EvUnspecified:       {EntityUnknown, "unspecified"},
	EvRunRequested:      {EntityRun, "requested"},
	EvRunStarted:        {EntityRun, "started"},
	EvRunCompleted:      {EntityRun, "completed"},
	EvRunFailed:         {EntityRun, "failed"},
	EvRunPartial:        {EntityRun, "partial"},
	EvHeartbeat:         {EntityRun, "heartbeat"},
	EvResourceStarted:   {EntityResource, "started"},
	EvPageFetched:       {EntityResource, "page_fetched"},
	EvResourceCompleted: {EntityResource, "completed"},
	EvResourceFailed:    {EntityResource, "failed"},
	EvBatchBuffered:     {EntityBatch, "buffered"},
	EvBatchWritten:      {EntityBatch, "written"},
	EvIntegrityVerified: {EntityBatch, "integrity_verified"},
	EvChunkDivergence:   {EntityBatch, "chunk_divergence"},
	EvWatermarkAdvanced: {EntityCursor, "watermark_advanced"},
	EvCheckpointSaved:   {EntityCursor, "checkpoint_saved"},
	EvRateLimited:       {EntityPressure, "rate_limited"},
	EvRetryExhausted:    {EntityPressure, "retry_exhausted"},
	EvScheduleFired:     {EntitySchedule, "fired"},
}

// orderedEntities is the canonical set of wire entities (excludes EntityUnknown),
// used for validation and for listing them in error messages.
var orderedEntities = []Entity{
	EntityRun, EntityResource, EntityBatch, EntityCursor, EntityPressure, EntitySchedule,
}

var entitySet = func() map[Entity]bool {
	m := make(map[Entity]bool, len(orderedEntities))
	for _, e := range orderedEntities {
		m[e] = true
	}
	return m
}()

var entityList = func() string {
	ss := make([]string, len(orderedEntities))
	for i, e := range orderedEntities {
		ss[i] = string(e)
	}
	return strings.Join(ss, ", ")
}()

// nameToEvent reverses eventMetaTable: "<entity>.<event>" → EventType.
var nameToEvent = func() map[string]EventType {
	m := make(map[string]EventType, len(eventMetaTable))
	for et, meta := range eventMetaTable {
		if et == EvUnspecified {
			continue
		}
		m[string(meta.entity)+subjectSep+meta.name] = et
	}
	return m
}()

// Entity returns the family this event type belongs to.
func (t EventType) Entity() Entity {
	if m, ok := eventMetaTable[t]; ok {
		return m.entity
	}
	return EntityUnknown
}

// EventName returns the leaf event token (e.g. "started", "page_fetched").
func (t EventType) EventName() string {
	if m, ok := eventMetaTable[t]; ok {
		return m.name
	}
	return "unspecified"
}

// String renders "<entity>.<event>" for logs (e.g. "run.started").
func (t EventType) String() string {
	m := eventMetaTable[t]
	return string(m.entity) + subjectSep + m.name
}

// Valid reports whether t is a known event type.
func (t EventType) Valid() bool {
	_, ok := eventMetaTable[t]
	return ok && t != EvUnspecified
}

// Subject builds the full publish subject for this event type + addressing.
func (t EventType) Subject(tenant TenantID, run RunID) string {
	m := eventMetaTable[t]
	return strings.Join(
		[]string{SubjectPrefix, SubjectVersion, string(m.entity), string(tenant), string(run), m.name},
		subjectSep,
	)
}

// Subject builds the publish subject for this concrete event.
//
// It does NOT validate: if Tenant or Run contain a '.', '*', '>', or whitespace
// the resulting subject is corrupt. Call Validate (or validate IDs at ingress)
// before publishing — the EventBus Publish path is expected to do so.
func (e Event) Subject() string { return e.Type.Subject(e.Tenant, e.Run) }

// Validate reports whether this event can be safely published: a known event
// type and routing tokens (tenant, run) that won't corrupt the subject.
func (e Event) Validate() error {
	if !e.Type.Valid() {
		return fmt.Errorf("%w: event type %d", ErrSubjectEvent, e.Type)
	}
	if err := e.Tenant.Valid(); err != nil {
		return fmt.Errorf("tenant %w", err)
	}
	if err := e.Run.Valid(); err != nil {
		return fmt.Errorf("run %w", err)
	}
	return nil
}

// ValidToken reports whether s is usable as a single subject token, wrapping
// [ErrInvalidToken] on the first problem. See [eventbus.ValidToken].
func ValidToken(s string) error { return eventbus.ValidToken(s) }

// IsValidToken is the boolean form of ValidToken.
func IsValidToken(s string) bool { return eventbus.IsValidToken(s) }

// Subject is a parsed subject — the routing tokens, without the payload.
type Subject struct {
	Version string
	Entity  Entity
	Tenant  TenantID
	Run     RunID
	Event   string
	Type    EventType // resolved from (Entity, Event); the inverse of EventType.Subject
}

// String reassembles the subject.
func (s Subject) String() string {
	return strings.Join(
		[]string{SubjectPrefix, s.Version, string(s.Entity), string(s.Tenant), string(s.Run), s.Event},
		subjectSep,
	)
}

// ParseSubject splits a concrete subject into its routing tokens, validating
// each one. On failure it returns an error wrapping a specific sentinel
// (ErrSubjectPrefix, ErrSubjectVersion, ErrSubjectEntity, …) plus a concrete
// message naming the offending token, so callers can branch with errors.Is.
func ParseSubject(s string) (Subject, error) {
	if s == "" {
		return Subject{}, fmt.Errorf("parse subject: %w: empty string", ErrSubjectTokenCount)
	}
	if i := strings.IndexAny(s, TokenWildcard+TailWildcard); i >= 0 {
		return Subject{}, fmt.Errorf("parse subject %q: %w (found %q at position %d)",
			s, ErrSubjectIsPattern, s[i:i+1], i)
	}

	toks := strings.Split(s, subjectSep)
	if len(toks) != subjectTokens {
		rel := "too few"
		if len(toks) > subjectTokens {
			rel = "too many"
		}
		return Subject{}, fmt.Errorf("parse subject %q: %w: want %d (%s), got %d (%s)",
			s, ErrSubjectTokenCount, subjectTokens, subjectShape, len(toks), rel)
	}

	sub := Subject{
		Version: toks[1],
		Entity:  Entity(toks[2]),
		Tenant:  TenantID(toks[3]),
		Run:     RunID(toks[4]),
		Event:   toks[5],
	}

	if toks[0] != SubjectPrefix {
		return Subject{}, fmt.Errorf("parse subject %q: %w: got %q, want %q",
			s, ErrSubjectPrefix, toks[0], SubjectPrefix)
	}
	if sub.Version != SubjectVersion {
		return Subject{}, fmt.Errorf("parse subject %q: %w: got %q, want %q",
			s, ErrSubjectVersion, sub.Version, SubjectVersion)
	}
	if !entitySet[sub.Entity] {
		return Subject{}, fmt.Errorf("parse subject %q: %w: got %q, want one of [%s]",
			s, ErrSubjectEntity, sub.Entity, entityList)
	}
	if sub.Tenant == "" {
		return Subject{}, fmt.Errorf("parse subject %q: %w: tenant is empty", s, ErrSubjectEmptyToken)
	}
	if sub.Run == "" {
		return Subject{}, fmt.Errorf("parse subject %q: %w: run is empty", s, ErrSubjectEmptyToken)
	}

	et, ok := nameToEvent[string(sub.Entity)+subjectSep+sub.Event]
	if !ok {
		return Subject{}, fmt.Errorf("parse subject %q: %w: %q is not a valid %q event",
			s, ErrSubjectEvent, sub.Event, sub.Entity)
	}
	sub.Type = et
	return sub, nil
}

// RunPattern matches every event of one run (any entity, any event).
func RunPattern(tenant TenantID, run RunID) string {
	return join(SubjectPrefix, SubjectVersion, TokenWildcard, string(tenant), string(run), TokenWildcard)
}

// TenantPattern matches every event of one tenant.
func TenantPattern(tenant TenantID) string {
	return join(SubjectPrefix, SubjectVersion, TokenWildcard, string(tenant), TokenWildcard, TokenWildcard)
}

// EntityPattern matches every event of one entity for one run (e.g. all batch.*).
func EntityPattern(tenant TenantID, run RunID, e Entity) string {
	return join(SubjectPrefix, SubjectVersion, string(e), string(tenant), string(run), TokenWildcard)
}

// AllPattern matches every ingestion fact in the v1 namespace.
func AllPattern() string {
	return join(SubjectPrefix, SubjectVersion, TailWildcard)
}

// EventPattern matches one event type across every tenant and run — e.g.
// EventPattern(EvRunRequested) → "ingestion.v1.run.*.*.requested". Modules that
// react to a single fact type (the engine on run.requested, the scheduler on
// schedule.fired) subscribe with this.
func EventPattern(t EventType) string {
	m := eventMetaTable[t]
	return join(SubjectPrefix, SubjectVersion, string(m.entity), TokenWildcard, TokenWildcard, m.name)
}

func join(toks ...string) string { return strings.Join(toks, subjectSep) }

// SubjectMatch reports whether a NATS-style pattern matches a concrete subject.
// See [eventbus.SubjectMatch].
func SubjectMatch(pattern, subject string) bool { return eventbus.SubjectMatch(pattern, subject) }

// JSONCodec marshals an Event for transport buses
var JSONCodec eventbus.Codec = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Encode(payload any) ([]byte, error) {
	ev, ok := payload.(Event)
	if !ok {
		return nil, fmt.Errorf("pkg: codec expected Event, got %T", payload)
	}
	return json.Marshal(ev)
}

func (jsonCodec) Decode(data []byte) (any, error) {
	var ev Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, fmt.Errorf("pkg: decode event: %w", err)
	}
	return ev, nil
}

// Subject parse errors. Each is wrapped (via %w) with concrete detail at the
// call site, so callers can both read a human message and branch with errors.Is
var (
	ErrSubjectIsPattern  = errors.New("subject is a pattern, not concrete")
	ErrSubjectTokenCount = errors.New("wrong token count")
	ErrSubjectPrefix     = errors.New("bad prefix")
	ErrSubjectVersion    = errors.New("unknown version")
	ErrSubjectEntity     = errors.New("unknown entity")
	ErrSubjectEmptyToken = errors.New("empty token")
	ErrSubjectEvent      = errors.New("unknown event")
	ErrSubjectPattern    = errors.New("malformed subject pattern")

	// ErrInvalidToken is returned when an ID (tenant/run/…) can't be used as a
	// subject token. Aliased to the shared sentinel so errors.Is matches across
	// the eventbus boundary.
	ErrInvalidToken = eventbus.ErrInvalidToken

	// ErrNotFound is the sentinel a DataStore returns (wrapped) when a run,
	// resource, or checkpoint does not exist, so callers can branch with
	// errors.Is — e.g. the tracker creating run state on first sight of a fact.
	ErrNotFound = errors.New("not found")
)

// subjectShape names the token layout, used in ErrSubjectTokenCount messages.
const subjectShape = "prefix.version.entity.tenant.run.event"
