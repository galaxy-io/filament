//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

type collectSink struct {
	mu   sync.Mutex
	recs []filament.Record
}

func (s *collectSink) Push(r filament.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r)
	return nil
}

func (s *collectSink) PushBatch(rs []filament.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, rs...)
	return nil
}

type flakySink struct {
	filament.Sink
	failAt int
	writes int
}

func (s *flakySink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return filament.WriteReceipt{}, fmt.Errorf("injected write failure at batch %d", s.writes)
	}
	return s.Sink.Apply(ctx, b, opts)
}

func waitStatus(t *testing.T, ctx context.Context, store filament.DataStore, id filament.RunID, want filament.RunStatus) filament.RunState {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, id)
		if err == nil && state.Status == want {
			return state
		}
		select {
		case <-ctx.Done():
			if err == nil {
				t.Fatalf("waiting for run %s status %v: last status %v error %q", id, want, state.Status, state.Error)
			}
			t.Fatalf("waiting for run %s status %v: %v", id, want, err)
		case <-ticker.C:
		}
	}
}
