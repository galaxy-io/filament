package streamkit

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type messageReceiver struct{ batches []*arrowbatch.Batch }

func (r *messageReceiver) Chunk(b *arrowbatch.Batch) error {
	r.batches = append(r.batches, b)
	return nil
}
func (*messageReceiver) Drained(rowmodel.Meta, int) error { return nil }

func TestMessageColumnsAndEventMetadata(t *testing.T) {
	payload := []byte(`{"event_id":"source-id","active":true,"amount":9007199254740993,"nested":{"a":[1,2]},"optional":null}`)
	base := rowmodel.Schema{Resource: "orders", Fields: []rowmodel.Field{{Name: "topic", Logical: rowmodel.LogicalString}}}
	columns, schema, err := NewMessageColumns(base, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := rowmodel.ValidateReservedFields(schema); err != nil {
		t.Fatal(err)
	}
	receiver := &messageReceiver{}
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer alloc.AssertSize(t, 0)
	writer := arrowbatch.NewBuilder(arrowbatch.Schema(schema), alloc, arrowbatch.Options{MaxRows: 100}, receiver)
	defer writer.Close()
	defer func() {
		for _, b := range receiver.batches {
			b.Release()
		}
	}()
	registry := &Registry{}
	if err := registry.Register("counter", 0, Uint64Codec{}); err != nil {
		t.Fatal(err)
	}
	projector := NewEventMetadataProjector(writer, registry)
	for _, data := range [][]byte{payload, []byte(`{"event_id":"next","active":false,"amount":1.25}`), nil} {
		appendPayload, err := columns.Prepare(data)
		if err != nil {
			t.Fatal(err)
		}
		writer.String("orders")
		appendPayload(writer)
		err = projector.EndEvent(Envelope{Identity: rowmodel.EventIdentity{Domain: rowmodel.DomainKey{Incarnation: "source", Domain: "orders"}, Position: rowmodel.Position{Codec: "counter", Value: []byte("1")}}, KeyNull: true}, rowmodel.Meta{})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	rows := receiver.batches[0].Rows()
	if len(rows.Schema().FieldIndices(rowmodel.EventPayloadField)) != 0 {
		t.Fatal("raw SDK payload leaked into decoded rows")
	}
	for name, want := range map[string]string{"event_id": "source-id", "amount": "9007199254740993", "nested": `{"a":[1,2]}`} {
		idx := rows.Schema().FieldIndices(name)[0]
		if got := rows.Column(idx).(*array.String).Value(0); got != want {
			t.Fatalf("%s: %s", name, got)
		}
		if !rows.Column(idx).IsNull(2) {
			t.Fatalf("tombstone %s is not null", name)
		}
	}
	if !rows.Column(rows.Schema().FieldIndices("nested")[0]).IsNull(1) {
		t.Fatal("missing field is not null")
	}
	if rows.Column(rows.Schema().FieldIndices(rowmodel.EventIDField)[0]).(*array.String).Value(0) == "source-id" {
		t.Fatal("source ID overwrote Filament identity")
	}
}

func TestMessageSchemaValidation(t *testing.T) {
	base := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "topic", Logical: rowmodel.LogicalString}}}
	for _, payload := range []string{`{"topic":"x"}`, `{"_FILAMENT_event_id":"x"}`, `{"a":1,"a":2}`, `{"A":1,"a":2}`, `{"":1}`} {
		if _, _, err := NewMessageColumns(base, []byte(payload)); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
	columns, _, err := NewMessageColumns(base, []byte(`{"name":"one","active":true}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{`{"name":2}`, `{"active":"true"}`, `{"new":1}`, `{"name":"x","name":"y"}`, `[]`, `invalid`} {
		if _, err := columns.Prepare([]byte(payload)); err == nil {
			t.Fatalf("accepted schema drift %s", payload)
		}
	}
}

func TestRawMessageFallback(t *testing.T) {
	columns, schema, err := NewMessageColumns(rowmodel.Schema{}, []byte{0xff, 0})
	if err != nil {
		t.Fatal(err)
	}
	if schema.Fields[0].Name != "payload" || schema.Fields[0].Logical != rowmodel.LogicalBytes {
		t.Fatal(schema)
	}
	if _, err := columns.Prepare([]byte(`{"a":1}`)); err == nil {
		t.Fatal("raw-to-object schema drift accepted")
	}
	for _, payload := range [][]byte{nil, {}, {0xff, 0}} {
		if _, err := columns.Prepare(payload); err != nil {
			t.Fatal(err)
		}
	}
	// Reserved metadata provenance survives cloning and rejects mutation.
	cloned := schema.Clone()
	cloned.Fields[len(cloned.Fields)-1].Name = "_filament_other"
	if rowmodel.ValidateReservedFields(cloned) == nil {
		t.Fatal("modified SDK metadata accepted")
	}
	if rowmodel.ValidateReservedFields(schema) != nil {
		t.Fatal("clone mutated original")
	}
}
