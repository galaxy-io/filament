package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/streamkit"
)

func TestEpochCompletionBindsCoverageAndAppliedResources(t *testing.T) {
	p := streamForTest(t, &fakeSink{}, filament.OrderingNone)
	ref := filament.EpochRef{Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "execution", StreamID: "stream", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}
	if err := p.BindEpoch(ref); err != nil {
		t.Fatal(err)
	}
	w := streamBuilder(t, p, "users")
	streamRow(t, w, 1)
	if _, err := p.SealEpoch(); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("unflushed seal: %v", err)
	}
	control := boundary(filament.ProgressBoundary)
	if _, err := p.Barrier(context.Background(), control); err != nil {
		t.Fatal(err)
	}
	proof, err := p.SealEpoch()
	if err != nil {
		t.Fatal(err)
	}
	rows, nbytes := proof.Totals()
	request := filament.EpochCommit{Completion: proof, Certificate: filament.EpochCertificate{Ref: ref, Coverage: filament.Coverage{Positions: filament.DomainPositions{control.Domain: control.Position}}, Records: rows, Bytes: nbytes, Receipts: []filament.EpochReceipt{{Resource: "users", Rows: rows, Bytes: nbytes}}}}
	codecs := &streamkit.Registry{}
	if err := codecs.Register("opaque", 0, streamkit.OpaqueCodec{}); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("applied rows=%d", rows)
	}
	if err := request.ValidateCompletion(codecs); err != nil {
		t.Fatal(err)
	}
	clone := proof.Positions()
	position := clone[control.Domain]
	position.Value[0] = 'X'
	if err := request.ValidateCompletion(codecs); err != nil {
		t.Fatal("proof accessor mutated evidence", err)
	}
	request.Certificate.Receipts[0].Resource = "other"
	if err := request.ValidateCompletion(codecs); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("substituted resource: %v", err)
	}
	request.Certificate.Receipts[0].Resource = "users"
	request.Certificate.Ref.Epoch = 2
	if err := request.ValidateCompletion(codecs); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("reused evidence: %v", err)
	}
	ref.Epoch++
	if err := p.BindEpoch(ref); err != nil {
		t.Fatal(err)
	}
	streamRow(t, w, 2)
	if _, err := p.Barrier(context.Background(), control); err != nil {
		t.Fatal(err)
	}
	next, err := p.SealEpoch()
	if err != nil {
		t.Fatal(err)
	}
	if next.Binding() == proof.Binding() {
		t.Fatal("epoch binding reused")
	}
	if rows, _ := next.Totals(); rows != 1 {
		t.Fatal("totals carried across epochs")
	}
}

func TestEpochSealInvalidatedByLaterRows(t *testing.T) {
	p := streamForTest(t, &fakeSink{}, filament.OrderingNone)
	ref := filament.EpochRef{Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "execution", StreamID: "stream", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}
	if err := p.BindEpoch(ref); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Barrier(context.Background(), boundary(filament.ProgressBoundary)); err != nil {
		t.Fatal(err)
	}
	streamRow(t, streamBuilder(t, p, "users"), 1)
	if _, err := p.SealEpoch(); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("stale control sealed: %v", err)
	}
}
