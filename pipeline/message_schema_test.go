package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	schemadrift "github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/streamkit"
)

func TestMessageLayoutChangesOnlyBeforeEpochRows(t *testing.T) {
	ctx := context.Background()
	preparations := 0
	p, err := NewStream(Config{Sink: &fakeSink{}, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullAppend)}, MessageSchemaPolicy: schemadrift.NullableAdd, PrepareSchema: func(context.Context, string, rowmodel.Schema) error { preparations++; return nil }}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(ctx)
	defer func() {
		p.CloseIngest(nil)
		if err := p.Wait(); err != nil {
			t.Error(err)
		}
	}()
	ref := filament.EpochRef{Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}
	records := p.Records().(filament.MessageSchemaRecordSink)
	base := rowmodel.Schema{Resource: "users"}
	var accepted *schemadrift.MessageLayout
	var first filament.EpochCompletion
	for epoch, payload := range []string{`{"name":"a"}`, `{"name":"b","extra":true}`, `{"name":"c"}`} {
		ref.Epoch = int64(epoch + 1)
		if err := p.BindEpoch(ref); err != nil {
			t.Fatal(err)
		}
		columns, model, err := streamkit.ProbeMessageBatch(base, accepted, [][]byte{[]byte(payload)}, schemadrift.NullableAdd)
		if err != nil {
			t.Fatal(err)
		}
		layout := columns.Layout()
		w, err := records.MessageBuilder(ctx, "users", model, layout)
		if err != nil {
			t.Fatal(err)
		}
		write, err := columns.Prepare([]byte(payload))
		if err != nil {
			t.Fatal(err)
		}
		write(w)
		for range 5 {
			w.Null()
		} // event metadata
		if err := w.EndRow(rowmodel.Meta{}); err != nil {
			t.Fatal(err)
		}
		if _, err := records.MessageBuilder(ctx, "users", model, layout); !errors.Is(err, filament.ErrEpochMismatch) {
			t.Fatalf("mid-epoch mutation: %v", err)
		}
		c := boundary(filament.ProgressBoundary)
		if err := records.Control(ctx, c); err != nil {
			t.Fatal(err)
		}
		proof, err := p.SealEpoch()
		if err != nil {
			t.Fatal(err)
		}
		if epoch == 0 {
			first = proof
		}
		if len(proof.MessageSchemas()["users"].Fields) != len(layout.Fields) {
			t.Fatal("layout not in completion")
		}
		accepted = &layout
	}
	if preparations != 2 {
		t.Fatalf("expected preparation per layout, got %d", preparations)
	}
	if len(first.MessageSchemas()["users"].Fields) != 1 {
		t.Fatal("later expansion mutated committed evidence")
	}
}
