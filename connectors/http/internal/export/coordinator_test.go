package export

import (
	"errors"
	"maps"
	"testing"
)

type coordinatorSink struct {
	rejectingSink
	tokens []string
	err    error
}

func (s *coordinatorSink) Checkpoint(_, token string) error {
	if s.err != nil {
		return s.err
	}
	s.tokens = append(s.tokens, token)
	return nil
}

func TestCoordinatorBatchesCompletionButPersistsIdentityChanges(t *testing.T) {
	_, res, state := deadlineFixture(t, "planned", "planned")
	sink := &coordinatorSink{}
	g := &exportCoordinator{state: state, target: sink, resource: res.Name}
	for _, key := range g.state.Selected {
		job := *g.state.Parents[key].Job
		job.Captures = maps.Clone(job.Captures)
		job.Phase = "creating"
		if err := g.update(key, job); err != nil {
			t.Fatal(err)
		}
		job.Phase = "waiting"
		if err := g.update(key, job); err != nil {
			t.Fatal(err)
		}
		before := len(sink.tokens)
		if err := g.update(key, job); err != nil {
			t.Fatal(err)
		}
		if len(sink.tokens) != before {
			t.Fatal("unchanged poll published a full snapshot")
		}
		job.Captures["url"] = "https://example.com/?signature=secret"
		if err := g.update(key, job); err != nil {
			t.Fatal(err)
		}
		if len(sink.tokens) != before+1 {
			t.Fatal("changed captures not published eagerly")
		}
		job.Phase = "done"
		if err := g.update(key, job); err != nil {
			t.Fatal(err)
		}
		if len(sink.tokens) != before+1 {
			t.Fatal("completion published immediately")
		}
	}
	if err := g.flush(); err != nil {
		t.Fatal(err)
	}
	if len(sink.tokens) != 7 {
		t.Fatalf("snapshots = %d, want 7", len(sink.tokens))
	}
	if err := g.flush(); err != nil {
		t.Fatal(err)
	}
	if len(sink.tokens) != 7 {
		t.Fatal("clean flush repeated snapshot")
	}
	// A failed eager publication must not make the coordinator believe the
	// new identity is saved, or suppress the retry as an unchanged update.
	job := *g.state.Parents[g.state.Selected[0]].Job
	job.Phase = "waiting"
	sink.err = errors.New("checkpoint unavailable")
	if err := g.update(g.state.Selected[0], job); !errors.Is(err, sink.err) {
		t.Fatal("lost publication error")
	}
	if g.state.Parents[g.state.Selected[0]].Job.Phase != "done" {
		t.Fatal("failed update was not rolled back")
	}
	sink.err = nil
	if err := g.update(g.state.Selected[0], job); err != nil {
		t.Fatal(err)
	}
	if len(sink.tokens) != 8 {
		t.Fatal("failed publication could not be retried")
	}
}

func TestCompletionFlushFailureIsRetried(t *testing.T) {
	_, res, state := deadlineFixture(t, "waiting")
	sink := &coordinatorSink{err: errors.New("unavailable")}
	g := &exportCoordinator{state: state, target: sink, resource: res.Name}
	job := *g.state.Parents[g.state.Selected[0]].Job
	job.Phase = "done"
	if err := g.update(g.state.Selected[0], job); err != nil {
		t.Fatal(err)
	}
	if err := g.flush(); !errors.Is(err, sink.err) {
		t.Fatal("flush failure was lost")
	}
	sink.err = nil
	if err := g.flush(); err != nil {
		t.Fatal(err)
	}
	if len(sink.tokens) != 1 {
		t.Fatal("failed completion flush was not retried")
	}
}
