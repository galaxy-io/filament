package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

type boundaryTestSink struct {
	filament.Sink
	reusable                     bool
	runs                         []filament.RunSpec
	begins, commits, closes      int
	openErr, commitErr, closeErr error
}

func (s *boundaryTestSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "test", Capabilities: filament.SinkCapabilities{ReusableSession: s.reusable}}
}

func (s *boundaryTestSink) Open(_ context.Context, r filament.RunSpec) error {
	s.runs = append(s.runs, r)
	return s.openErr
}
func (s *boundaryTestSink) Begin(context.Context) error  { s.begins++; return nil }
func (s *boundaryTestSink) Commit(context.Context) error { s.commits++; return s.commitErr }
func (s *boundaryTestSink) Abort(context.Context) error  { return nil }
func (s *boundaryTestSink) Close(context.Context) error  { s.closes++; return s.closeErr }

type boundaryTestSchemaSink struct {
	boundaryTestSink
	schemas map[string]rowmodel.Schema
}

func (s *boundaryTestSchemaSink) EnsureSchema(_ context.Context, r string, m rowmodel.Schema) error {
	if s.schemas == nil {
		s.schemas = map[string]rowmodel.Schema{}
	}
	s.schemas[r] = m
	return nil
}

func TestBoundarySinkFreshInstancesReplaySchema(t *testing.T) {
	ctx := context.Background()
	first := &boundaryTestSchemaSink{}
	second := &boundaryTestSchemaSink{}
	calls := 0
	s := newBoundarySink(first, func() (filament.Sink, error) { calls++; return second, nil })
	run := filament.RunSpec{Run: "source-run", ExecutionID: "owner", StreamAttempt: &filament.AttemptRef{}, ReplicationStream: &filament.StreamRef{}}
	run.Options.Execution = filament.ExecutionContinuous
	if err := s.Open(ctx, run); err != nil {
		t.Fatal(err)
	}
	model := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}
	if err := s.(filament.Schematized).EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	model.Fields[0].Name = "mutated"
	for range 2 {
		if err := s.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 || first.closes != 1 || second.closes != 1 {
		t.Fatalf("factory=%d closes=%d/%d", calls, first.closes, second.closes)
	}
	if second.schemas["items"].Fields[0].Name != "id" {
		t.Fatal("schema not cloned and replayed")
	}
	a, b := first.runs[0], second.runs[0]
	if a.Run != run.Run || b.Run != run.Run || a.SinkWriteID == b.SinkWriteID || a.SinkWriteID == "" || b.SinkWriteID == "" {
		t.Fatal("logical run changed or artifact identities were reused")
	}
	if a.SinkWriteStartedAt.IsZero() || b.SinkWriteStartedAt.Before(a.SinkWriteStartedAt) {
		t.Fatal("write time not refreshed")
	}
	for _, r := range []filament.RunSpec{a, b} {
		if r.Options.Execution != filament.ExecutionBounded || r.StreamAttempt != nil || r.ReplicationStream != nil || r.ExecutionID != "" {
			t.Fatalf("source ownership leaked: %+v", r)
		}
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if second.closes != 1 {
		t.Fatal("duplicate close")
	}
}

func TestBoundarySinkReusableSession(t *testing.T) {
	ctx := context.Background()
	dst := &boundaryTestSink{reusable: true}
	s := newBoundarySink(dst, nil)
	if _, ok := s.(filament.Schematized); ok {
		t.Fatal("invented schema capability")
	}
	if err := s.Open(ctx, filament.RunSpec{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(dst.runs) != 1 || dst.begins != 2 || dst.closes != 0 {
		t.Fatal("reusable session was reopened or closed early")
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if dst.closes != 1 {
		t.Fatal("session not closed")
	}
}

func TestBoundarySinkRetainsFailures(t *testing.T) {
	boom := errors.New("uncertain destination outcome")
	ctx := context.Background()
	for _, phase := range []string{"open", "commit", "close"} {
		t.Run(phase, func(t *testing.T) {
			dst := &boundaryTestSink{}
			if phase == "open" {
				dst.openErr = boom
			}
			if phase == "commit" {
				dst.commitErr = boom
			}
			if phase == "close" {
				dst.closeErr = boom
			}
			s := newBoundarySink(dst, nil)
			err := s.Open(ctx, filament.RunSpec{})
			if phase == "open" {
				if !errors.Is(err, boom) {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Begin(ctx); err != nil {
					t.Fatal(err)
				}
				if err = s.Commit(ctx); !errors.Is(err, boom) {
					t.Fatalf("lost commit/close failure: %v", err)
				}
			}
			err = s.Close(ctx)
			if phase == "close" && !errors.Is(err, boom) {
				t.Fatalf("lost failure on Close: %v", err)
			}
			if phase != "close" && err != nil {
				t.Fatalf("write error reported as failed cleanup: %v", err)
			}
			if dst.closes != 1 {
				t.Fatalf("closes=%d", dst.closes)
			}
		})
	}
}

func TestBoundarySchemaBetweenWrites(t *testing.T) {
	ctx := context.Background()
	first := &boundaryTestSchemaSink{}
	second := &boundaryTestSchemaSink{}
	s := newBoundarySink(first, func() (filament.Sink, error) { return second, nil })
	if err := s.Open(ctx, filament.RunSpec{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	model := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "late", Logical: rowmodel.LogicalString}}}
	if err := s.(filament.Schematized).EnsureSchema(ctx, "late", model); err != nil {
		t.Fatal(err)
	}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	if second.schemas["late"].Fields[0].Name != "late" {
		t.Fatal("schema was not replayed")
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

type concurrentSchemaSink struct {
	boundaryTestSink
	entered chan string
	release chan struct{}
}

func (s *concurrentSchemaSink) EnsureSchema(_ context.Context, r string, _ rowmodel.Schema) error {
	s.entered <- r
	<-s.release
	return nil
}

func TestBoundarySchemaDoesNotSerializeDDL(t *testing.T) {
	dst := &concurrentSchemaSink{entered: make(chan string, 2), release: make(chan struct{})}
	done := make(chan struct{}, 2)
	defer func() { close(dst.release); <-done; <-done }()
	s := newBoundarySink(dst, nil)
	if err := s.Open(context.Background(), filament.RunSpec{}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"a", "b"} {
		go func() {
			_ = s.(filament.Schematized).EnsureSchema(context.Background(), r, rowmodel.Schema{})
			done <- struct{}{}
		}()
	}
	for range 2 {
		select {
		case <-dst.entered:
		case <-time.After(time.Second):
			t.Fatal("independent schema setup serialized")
		}
	}
}
