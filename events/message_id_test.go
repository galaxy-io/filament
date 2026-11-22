package events

import (
	"testing"
	"time"
)

func TestFactMessageIDIsStableAndDistinct(t *testing.T) {
	at := time.Date(2026, 11, 18, 21, 21, 8, 0, time.UTC)
	base := Envelope{Tenant: "t1", Run: "run1", Seq: 1, At: at}
	started := NewFact(RunStarted, base, RunStartedEvent{})

	if started.MessageID() == "" {
		t.Fatal("a stamped fact must carry a message ID")
	}
	if started.MessageID() != NewFact(RunStarted, base, RunStartedEvent{}).MessageID() {
		t.Fatal("republishing the same fact must reuse its message ID")
	}

	resumed := base
	resumed.At = at.Add(time.Second)
	otherRun := base
	otherRun.Run = "run2"
	otherSeq := base
	otherSeq.Seq = 2
	otherResource := base
	otherResource.Resource = "orders"
	for name, other := range map[string]Fact{
		"resumed attempt reusing the sequence": NewFact(RunStarted, resumed, RunStartedEvent{}),
		"another run":                          NewFact(RunStarted, otherRun, RunStartedEvent{}),
		"next sequence":                        NewFact(RunStarted, otherSeq, RunStartedEvent{}),
		"another resource":                     NewFact(RunStarted, otherResource, RunStartedEvent{}),
		"another event":                        NewFact(RunCompleted, base, RunCompletedEvent{}),
	} {
		if other.MessageID() == started.MessageID() {
			t.Fatalf("%s shares message ID %q", name, started.MessageID())
		}
	}
}

func TestFactWithoutEmitTimeOptsOutOfMessageID(t *testing.T) {
	f := NewFact(RunStarted, Envelope{Tenant: "t1", Run: "run1"}, RunStartedEvent{})
	if id := f.MessageID(); id != "" {
		t.Fatalf("MessageID = %q, want empty without an emit time", id)
	}
}
