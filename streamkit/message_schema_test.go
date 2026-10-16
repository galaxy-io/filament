package streamkit

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
	schemadrift "github.com/galaxy-io/filament/schema"
)

func TestProbeMessageBatchUnionAndRestore(t *testing.T) {
	base := rowmodel.Schema{Resource: "events", Fields: []rowmodel.Field{{Name: "subject", Logical: rowmodel.LogicalString}}}
	columns, _, err := ProbeMessageBatch(base, nil, [][]byte{[]byte(`{"id":1}`), []byte(`{"id":2,"email":"a@example.com"}`)}, schemadrift.NullableAdd)
	if err != nil {
		t.Fatal(err)
	}
	accepted := columns.Layout()
	if len(accepted.Fields) != 2 {
		t.Fatalf("batch union: %+v", accepted)
	}
	for _, f := range accepted.Fields {
		if !f.Nullable {
			t.Fatal("inferred required field")
		}
	}
	restored, model, err := ProbeMessageBatch(base, &accepted, [][]byte{[]byte(`{"id":3}`)}, schemadrift.NullableAdd)
	if err != nil || len(restored.Layout().Fields) != 2 || len(model.Fields) != 8 {
		t.Fatalf("omitted field lost on restart: %+v %v", model, err)
	}
	expanded, _, err := ProbeMessageBatch(base, &accepted, [][]byte{[]byte(`{"id":4,"extra":null}`), []byte(`{"extra":"value"}`)}, schemadrift.NullableAdd)
	if err != nil {
		t.Fatal(err)
	}
	if expanded.Layout().Fields[2].Logical != rowmodel.LogicalJSON {
		t.Fatal("null-only field should use JSON")
	}
	if len(accepted.Fields) != 2 {
		t.Fatal("probe mutated accepted state")
	}
}

func TestProbeMessageBatchBlocksBeforeEncoding(t *testing.T) {
	base := rowmodel.Schema{Resource: "events"}
	initial, _, err := ProbeMessageBatch(base, nil, [][]byte{[]byte(`{"name":"a"}`)}, schemadrift.NullableAdd)
	if err != nil {
		t.Fatal(err)
	}
	accepted := initial.Layout()
	for _, tc := range []struct {
		name, payload string
		policy        schemadrift.Policy
	}{
		{"strict addition", `{"name":"b","extra":true}`, schemadrift.Strict},
		{"type change", `{"name":12}`, schemadrift.NullableAdd},
		{"reserved", `{"_filament_bad":1}`, schemadrift.NullableAdd},
		{"case collision", `{"Name":"x"}`, schemadrift.NullableAdd},
		{"raw change", `raw bytes`, schemadrift.NullableAdd},
		{"duplicate", `{"a":1,"a":2}`, schemadrift.NullableAdd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := ProbeMessageBatch(base, &accepted, [][]byte{[]byte(`{"name":"valid"}`), []byte(tc.payload)}, tc.policy); !errors.Is(err, schemadrift.ErrBlocked) {
				t.Fatalf("probe did not block: %v", err)
			}
		})
	}
	// Strict allows the initial whole-batch schema, including later sample fields.
	if _, _, err := ProbeMessageBatch(base, nil, [][]byte{[]byte(`{}`), []byte(`{"name":"a"}`)}, schemadrift.Strict); err != nil {
		t.Fatal(err)
	}
}
