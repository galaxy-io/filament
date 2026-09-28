package streamkit

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type controlSink struct {
	controls []rowmodel.Control
	err      error
}

func (*controlSink) Builder(string, int, rowmodel.Schema) (arrowbatch.RowWriter, error) {
	return nil, errors.New("unused")
}

func (s *controlSink) Control(_ context.Context, c rowmodel.Control) error {
	s.controls = append(s.controls, c.Clone())
	return s.err
}

func testControl(kind rowmodel.ControlKind) rowmodel.Control {
	c := rowmodel.Control{Kind: kind, Domain: rowmodel.DomainKey{Incarnation: "source", Domain: "log"}, Position: rowmodel.Position{Codec: "opaque", Value: []byte("1")}}
	if kind == rowmodel.TxnBegin || kind == rowmodel.TxnEnd {
		c.TxnID = "txn"
	}
	return c
}

func tracker(t *testing.T, limits filament.Boundary) *BoundaryTracker {
	t.Helper()
	b, err := NewBoundaryTracker(limits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

func add(t *testing.T, b *BoundaryTracker, records int, nbytes int64) {
	t.Helper()
	if err := b.Add(records, nbytes); err != nil {
		t.Fatal(err)
	}
}

func TestBoundaryLimits(t *testing.T) {
	for name, limits := range map[string]filament.Boundary{"records": {MaxRecords: 2}, "bytes": {MaxBytes: 10}} {
		t.Run(name, func(t *testing.T) {
			b := tracker(t, limits)
			add(t, b, 1, 5)
			if b.Requested() {
				t.Fatal("early request")
			}
			add(t, b, 1, 5)
			if !b.Requested() {
				t.Fatal("limit missed")
			}
			ok, err := b.Control(context.Background(), &controlSink{}, testControl(rowmodel.ProgressBoundary))
			if err != nil || !ok {
				t.Fatalf("completion: %v %v", ok, err)
			}
			if !errors.Is(b.Add(1, 1), ErrBoundaryClosed) {
				t.Fatal("accepted rows after boundary")
			}
		})
	}
}

func TestBoundaryIdleAndAge(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := tracker(t, filament.Boundary{MaxWait: 5 * time.Second, MaxAge: 8 * time.Second})
		time.Sleep(4 * time.Second)
		add(t, b, 1, 1)
		// Activity moves idle to t=9 but cannot move age beyond t=8.
		<-b.Wake()
		if !b.Requested() {
			t.Fatal("age wake did not request boundary")
		}
		idle := tracker(t, filament.Boundary{MaxWait: time.Second})
		<-idle.Wake()
		if !idle.Requested() {
			t.Fatal("idle wake without rows was lost")
		}
		add(t, idle, 1, 1)
		if !idle.Requested() {
			t.Fatal("new row cleared pending idle request")
		}
	})
}

func TestBoundaryTransactions(t *testing.T) {
	b := tracker(t, filament.Boundary{MaxRecords: 1})
	sink := &controlSink{}
	if ok, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnBegin)); err != nil || ok {
		t.Fatalf("begin: %v %v", ok, err)
	}
	add(t, b, 2, 20)
	if !b.Requested() {
		t.Fatal("limit should request, not truncate")
	}
	if ok, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnEnd)); err != nil || !ok {
		t.Fatalf("end: %v %v", ok, err)
	}
	if len(sink.controls) != 2 {
		t.Fatal("missing empty/standalone controls")
	}
}

func TestBoundaryIncompleteTransactionAndCancellation(t *testing.T) {
	for _, mode := range []string{"progress", "cancel", "wrong-end"} {
		t.Run(mode, func(t *testing.T) {
			b := tracker(t, filament.Boundary{MaxRecords: 1})
			sink := &controlSink{}
			if _, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnBegin)); err != nil {
				t.Fatal(err)
			}
			add(t, b, 1, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := testControl(rowmodel.ProgressBoundary)
			if mode == "cancel" {
				cancel()
				c = testControl(rowmodel.TxnEnd)
			}
			if mode == "wrong-end" {
				c = testControl(rowmodel.TxnEnd)
				c.TxnID = "other"
			}
			if ok, err := b.Control(ctx, sink, c); err == nil || ok {
				t.Fatal("invalid boundary completed")
			}
			if len(sink.controls) != 1 {
				t.Fatal("invalid control forwarded")
			}
			if ok, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnEnd)); err == nil || ok {
				t.Fatal("failed tracker recovered")
			}
		})
	}
}

func TestBoundaryControlFailure(t *testing.T) {
	b := tracker(t, filament.Boundary{MaxRecords: 1})
	add(t, b, 1, 1)
	failure := errors.New("flush failed")
	if ok, err := b.Control(context.Background(), &controlSink{err: failure}, testControl(rowmodel.ProgressBoundary)); ok || !errors.Is(err, failure) {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestBoundaryDisabledAndInvalid(t *testing.T) {
	b := tracker(t, filament.Boundary{})
	add(t, b, 100, 100)
	if b.Wake() != nil || b.Requested() {
		t.Fatal("disabled limits requested")
	}
	for _, limits := range []filament.Boundary{{MaxRecords: -1}, {MaxBytes: -1}, {MaxWait: -1}, {MaxAge: -1}} {
		if _, err := NewBoundaryTracker(limits); err == nil {
			t.Fatal("negative limit accepted")
		}
	}
	if err := b.Add(-1, 0); err == nil {
		t.Fatal("negative accounting accepted")
	}
}

func TestBoundaryExplicitRequestAndEmptyTransaction(t *testing.T) {
	b := tracker(t, filament.Boundary{})
	sink := &controlSink{}
	if _, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnBegin)); err != nil {
		t.Fatal(err)
	}
	if err := b.Request(); err != nil {
		t.Fatal(err)
	}
	if ok, err := b.Control(context.Background(), sink, testControl(rowmodel.TxnEnd)); err != nil || !ok {
		t.Fatalf("empty transaction: %v %v", ok, err)
	}
}

func TestBoundaryWaitsForAllOpenDomains(t *testing.T) {
	b := tracker(t, filament.Boundary{MaxRecords: 1})
	sink := &controlSink{}
	a, other := testControl(rowmodel.TxnBegin), testControl(rowmodel.TxnBegin)
	other.Domain.Domain = "other"
	for _, c := range []rowmodel.Control{a, other} {
		if _, err := b.Control(context.Background(), sink, c); err != nil {
			t.Fatal(err)
		}
	}
	add(t, b, 1, 1)
	a.Kind, other.Kind = rowmodel.TxnEnd, rowmodel.TxnEnd
	if ok, err := b.Control(context.Background(), sink, a); err != nil || ok {
		t.Fatalf("completed with another transaction open: %v %v", ok, err)
	}
	if ok, err := b.Control(context.Background(), sink, other); err != nil || !ok {
		t.Fatalf("last transaction: %v %v", ok, err)
	}
}
