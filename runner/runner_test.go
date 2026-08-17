package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
)

type commitTestSource struct{ incrementalTestSource }

func (*commitTestSource) PlanResume(context.Context, []string, map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	return nil, nil
}

// commitFailSink completes every write, then fails Commit, recording Abort calls.
type commitFailSink struct {
	incrementalTestSink
	aborts atomic.Int32
}

func (s *commitFailSink) Commit(context.Context) error { return errors.New("commit boom") }
func (s *commitFailSink) Abort(context.Context) error  { s.aborts.Add(1); return nil }

func TestRunOneCommitFailureAborts(t *testing.T) {
	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()

	failed := make(chan string, 1)
	resourceFailed := make(chan string, 1)
	go func() {
		for msg := range facts.C() {
			f, decodeErr := events.Decode(msg)
			_ = msg.Ack()
			if decodeErr != nil {
				continue
			}
			if d, ok := f.Data.(events.RunFailedEvent); ok {
				failed <- d.Error
				return
			}
			if d, ok := f.Data.(events.ResourceFailedEvent); ok && f.Resource == "users" {
				resourceFailed <- d.Error
			}
		}
	}()

	sink := &commitFailSink{}
	sources := registry.NewSources()
	sources.Register("test", func() filament.Source { return &commitTestSource{} })
	sinks := registry.NewSinks()
	sinks.Register("test-sink", func() filament.Sink { return sink })

	RunOne(context.Background(), Deps{
		Bus:       bus,
		DataStore: memory.New(),
		Sources:   sources,
		Sinks:     sinks,
	}, filament.RunSpec{
		Tenant:         "t1",
		Run:            "r1",
		Source:         filament.Ref{Provider: "test"},
		Sink:           filament.Ref{Provider: "test-sink"},
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
	})

	select {
	case msg := <-failed:
		if !strings.Contains(msg, "commit") {
			t.Fatalf("run.failed error = %q, want commit failure", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no run.failed fact published")
	}
	select {
	case msg := <-resourceFailed:
		if !strings.Contains(msg, "commit") {
			t.Fatalf("resource.failed error = %q, want commit failure", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no resource.failed fact published")
	}
	if got := sink.aborts.Load(); got != 1 {
		t.Fatalf("Abort calls = %d, want 1", got)
	}
}
