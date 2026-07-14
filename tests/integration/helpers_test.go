//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ingestion "github.com/galaxy-io/filament"
)

type collectSink struct {
	mu   sync.Mutex
	recs []ingestion.Record
}

func (s *collectSink) Push(r ingestion.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, r)
	return nil
}

func (s *collectSink) PushBatch(rs []ingestion.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = append(s.recs, rs...)
	return nil
}

type flakySink struct {
	ingestion.Sink
	failAt int
	writes int
}

func (s *flakySink) Apply(ctx context.Context, b ingestion.Batch, opts ingestion.ApplyOptions) (ingestion.WriteReceipt, error) {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return ingestion.WriteReceipt{}, fmt.Errorf("injected write failure at batch %d", s.writes)
	}
	return s.Sink.Apply(ctx, b, opts)
}

func waitStatus(t *testing.T, ctx context.Context, store ingestion.DataStore, id ingestion.RunID, want ingestion.RunStatus) ingestion.RunState {
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
