package httpapi

import (
	"sync"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// guardTestSink records pushed records and drained checkpoints.
type guardTestSink struct {
	mu          sync.Mutex
	records     []record
	checkpoints map[string][]string
}

func (s *guardTestSink) Push(rec record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	return nil
}

func (s *guardTestSink) PushBatch(records []record) error {
	for _, rec := range records {
		if err := s.Push(rec); err != nil {
			return err
		}
	}
	return nil
}

func (s *guardTestSink) Checkpoint(resource, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checkpoints == nil {
		s.checkpoints = map[string][]string{}
	}
	s.checkpoints[resource] = append(s.checkpoints[resource], token)
	return nil
}

func (s *guardTestSink) drained(resource string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkpoints[resource]
}

func timeGuardResource() manifest.Resource {
	return manifest.Resource{
		Incremental: &manifest.IncrementalSpec{Comparator: "time"},
	}
}

func TestReadGuardSharedDrainTakesMaxAcrossParents(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 0, timeGuardResource(), true)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	// Keys arrive out of order, as they do when two parents fan out
	// concurrently. The drained key must be the maximum, not the last push.
	keys := []string{"2026-01-02T00:00:00Z", "2026-01-04T00:00:00Z", "2026-01-03T00:00:00Z"}
	for i, key := range keys {
		if err := guard.Push(record{Resource: "actions", ID: string(rune('a' + i)), Key: []string{key}}); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
	}
	for _, rec := range sink.records {
		if len(rec.Key) != 0 {
			t.Fatalf("pushed record key = %v, want stripped keys while complete-only", rec.Key)
		}
	}
	if err := guard.drain("actions"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("actions"); len(got) != 1 || got[0] != "2026-01-04T00:00:00Z" {
		t.Fatalf("drained = %v, want [2026-01-04T00:00:00Z]", got)
	}
}

func TestReadGuardDrainSkippedWhenLimited(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 2, timeGuardResource(), true)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	if err := guard.Push(record{Resource: "actions", ID: "a", Key: []string{"2026-01-02T00:00:00Z"}}); err != nil {
		t.Fatalf("push 1: %v", err)
	}
	if err := guard.Push(record{Resource: "actions", ID: "b", Key: []string{"2026-01-03T00:00:00Z"}}); err == nil {
		t.Fatalf("push 2: want errReadLimit, got nil")
	} else if err != errReadLimit {
		t.Fatalf("push 2: err = %v, want errReadLimit", err)
	}
	if err := guard.drain("actions"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("actions"); len(got) != 0 {
		t.Fatalf("drained = %v, want nothing after a limited read", got)
	}
}

func TestReadGuardDrainSkippedWhenEmpty(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 0, timeGuardResource(), true)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	if err := guard.drain("actions"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("actions"); len(got) != 0 {
		t.Fatalf("drained = %v, want nothing with no records", got)
	}
	// Records with no key also drain nothing.
	if err := guard.Push(record{Resource: "actions", ID: "a"}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := guard.drain("actions"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("actions"); len(got) != 0 {
		t.Fatalf("drained = %v, want nothing with no key observed", got)
	}
}

func TestReadGuardConcurrentPushes(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 0, timeGuardResource(), true)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	var wg sync.WaitGroup
	for p := 0; p < 5; p++ {
		wg.Add(1)
		go func(parent int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				rec := record{
					Resource: "actions",
					ID:       string(rune('a'+parent)) + string(rune('0'+i)),
					Key:      []string{"2026-01-0" + string(rune('1'+parent)) + "T00:00:00Z"},
				}
				if err := guard.Push(rec); err != nil {
					t.Errorf("push: %v", err)
					return
				}
			}
		}(p)
	}
	wg.Wait()
	if len(sink.records) != 100 {
		t.Fatalf("records = %d, want 100", len(sink.records))
	}
	if err := guard.drain("actions"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("actions"); len(got) != 1 || got[0] != "2026-01-05T00:00:00Z" {
		t.Fatalf("drained = %v, want [2026-01-05T00:00:00Z]", got)
	}
}

func TestReadGuardTopLevelDrainUnchanged(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 0, timeGuardResource(), true)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	// A single ordered stream drains its last key, which is also its max.
	for i, key := range []string{"2026-01-02T00:00:00Z", "2026-01-03T00:00:00Z"} {
		if err := guard.Push(record{Resource: "items", ID: string(rune('a' + i)), Key: []string{key}}); err != nil {
			t.Fatalf("push %d: %v", i, err)
		}
	}
	if err := guard.drain("items"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("items"); len(got) != 1 || got[0] != "2026-01-03T00:00:00Z" {
		t.Fatalf("drained = %v, want [2026-01-03T00:00:00Z]", got)
	}
}

func TestReadGuardWithoutCompleteOnlyPassesKeysThrough(t *testing.T) {
	sink := &guardTestSink{}
	guard, err := newReadGuard(sink, 0, timeGuardResource(), false)
	if err != nil {
		t.Fatalf("newReadGuard: %v", err)
	}
	if err := guard.Push(record{Resource: "items", ID: "a", Key: []string{"2026-01-02T00:00:00Z"}}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if got := sink.records[0].Key; len(got) != 1 || got[0] != "2026-01-02T00:00:00Z" {
		t.Fatalf("pushed record key = %v, want it passed through", got)
	}
	if err := guard.drain("items"); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if got := sink.drained("items"); len(got) != 0 {
		t.Fatalf("drained = %v, want nothing without complete-only", got)
	}
}
