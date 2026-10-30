package streamkit

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

type negotiatingRecords struct {
	filament.StreamRecordSink
	calls int
	err   error
}

func (r *negotiatingRecords) MessageBuilder(context.Context, string, rowmodel.Schema, schema.MessageLayout) (filament.RowWriter, error) {
	r.calls++
	return nil, r.err
}

func TestMessageBatchesAdmissionAndRestore(t *testing.T) {
	base := func(resource string) rowmodel.Schema { return rowmodel.Schema{Resource: resource} }
	opts := filament.StreamOpenOpts{Resources: []string{"events"}, MessageSchemaPolicy: schema.NullableAdd}
	b, err := NewMessageBatches(opts, base)
	if err != nil {
		t.Fatal(err)
	}
	out := &negotiatingRecords{}
	var observations []schema.Detection
	ctx := schema.WithObserver(t.Context(), func(_ string, d schema.Detection) { observations = append(observations, d) })
	prepare := func(payloads ...string) error {
		data := make([][]byte, len(payloads))
		for i, p := range payloads {
			data[i] = []byte(p)
		}
		_, _, err := b.Prepare(ctx, out, base("events"), data)
		return err
	}
	if err := prepare(`{"id":1}`, `{"id":2,"email":"x"}`); err != nil {
		t.Fatal(err)
	}
	if len(b.layouts["events"].Fields) != 2 || len(observations) != 0 {
		t.Fatal("initial batch not established")
	}
	// Every message must validate before even asking the pipeline for a writer.
	if err := prepare(`{"id":3}`, `{"email":99}`); !errors.Is(err, schema.ErrBlocked) || out.calls != 1 {
		t.Fatalf("partial admission: %v, %d", err, out.calls)
	}
	out.err = filament.ErrEpochMismatch
	if err := prepare(`{"extra":true}`); !errors.Is(err, filament.ErrEpochMismatch) {
		t.Fatal(err)
	}
	if len(b.layouts["events"].Fields) != 2 {
		t.Fatal("failed negotiation advanced layout")
	}
	out.err = nil
	if err := prepare(`{"extra":true}`); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 2 || observations[0].Side != "source" || observations[0].Changes[0].Field != "extra" {
		t.Fatalf("missing tentative detection: %+v", observations)
	}
	opts.MessageSchemas = b.layouts
	restored, err := NewMessageBatches(opts, base)
	if err != nil {
		t.Fatal(err)
	}
	columns, _, err := restored.Prepare(ctx, out, base("events"), [][]byte{[]byte(`{"id":4}`)})
	if err != nil || len(columns.Layout().Fields) != 3 {
		t.Fatalf("restoration lost absent fields: %v", err)
	}
	b.layouts["events"].Fields[0].Name = "mutated"
	if restored.layouts["events"].Fields[0].Name == "mutated" {
		t.Fatal("restoration aliases caller state")
	}
}

func TestMessageBatchesStrictDetection(t *testing.T) {
	base := func(resource string) rowmodel.Schema { return rowmodel.Schema{Resource: resource} }
	b, err := NewMessageBatches(filament.StreamOpenOpts{Resources: []string{"events"}, MessageSchemaPolicy: schema.Strict, MessageSchemas: map[string]schema.MessageLayout{"events": {}}}, base)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ctx := schema.WithObserver(t.Context(), func(string, schema.Detection) { count++ })
	out := &negotiatingRecords{}
	_, _, err = b.Prepare(ctx, out, base("events"), [][]byte{[]byte(`{"new":true}`)})
	if !errors.Is(err, schema.ErrBlocked) || count != 1 || out.calls != 0 {
		t.Fatalf("strict detection/admission: %v %d %d", err, count, out.calls)
	}
}
