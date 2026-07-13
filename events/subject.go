package events

import (
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
)

// Subject grammar — the addressing scheme every fact travels under.
//
//	ingestion.v1.<entity>.<tenant>.<run>.<event>
//	e.g. ingestion.v1.run.T123.R789.started
//	     ingestion.v1.resource.T123.R789.page_fetched
//
// Exactly six dot-separated tokens. The resource name is not in the subject.
const (
	SubjectPrefix  = "ingestion"
	SubjectVersion = "v1"
)

// Subject builds the concrete publish subject for one fact:
// ingestion.v1.<entity>.<tenant>.<run>.<event>.
func Subject[T any](t EventType[T], tenant ingestion.TenantID, run ingestion.RunID) string {
	return join(t.entity, string(tenant), string(run), t.name)
}

// SubjectPattern is the subscription pattern matching every fact of one kind:
// ingestion.v1.<entity>.*.*.<event>. Modules that react to a single fact type
// (the engine on run.requested, the scheduler on schedule.fired) subscribe
// with this.
func SubjectPattern[T any](t EventType[T]) string {
	return join(t.entity, eventbus.TokenWildcard, eventbus.TokenWildcard, t.name)
}

// RunPattern matches every fact of one run (any kind).
func RunPattern(tenant ingestion.TenantID, run ingestion.RunID) string {
	return join(eventbus.TokenWildcard, string(tenant), string(run), eventbus.TokenWildcard)
}

// AllPattern matches every fact in the versioned namespace.
func AllPattern() string {
	return strings.Join([]string{SubjectPrefix, SubjectVersion, eventbus.TailWildcard}, eventbus.Separator)
}

// join assembles prefix.version.<entity>.<tenant>.<run>.<event>.
func join(entity, tenant, run, event string) string {
	return strings.Join([]string{SubjectPrefix, SubjectVersion, entity, tenant, run, event}, eventbus.Separator)
}
