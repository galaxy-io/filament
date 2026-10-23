package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/streamproof"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func TestRuntimeMessageLayoutsCannotRegressOrDowngrade(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	layout := schema.MessageLayout{Fields: []rowmodel.Field{{Name: "name", Logical: rowmodel.LogicalString, Nullable: true}}}
	request := func(epoch int64, position string, layouts map[string]schema.MessageLayout) filament.EpochCommit {
		r := f.epoch(t, a, epoch, position)
		r.Certificate.FormatVersion = filament.EpochMessageSchemaFormatVersion
		r.Certificate.MessageSchemas = schema.CloneMessageLayouts(layouts)
		rows, nbytes := r.Completion.Totals()
		r.Completion = streamproof.NewWithMessageSchemas(r.Completion.Binding(), r.Completion.Positions(), rows, nbytes, r.Completion.Resources(), layouts)
		return r
	}
	first := request(1, "1", map[string]schema.MessageLayout{"events": layout})
	if _, err := f.store.CommitEpoch(ctx, first); err != nil {
		t.Fatal(err)
	}
	expanded := layout.Clone()
	expanded.Fields = append(expanded.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalBool, Nullable: true})
	second := request(2, "2", map[string]schema.MessageLayout{"events": expanded})
	if _, err := f.store.CommitEpoch(ctx, second); err != nil {
		t.Fatal(err)
	}
	// Historical replay of an identical certificate retains the accepted layout.
	second.Completion = filament.EpochCompletion{}
	if _, err := f.store.CommitEpoch(ctx, second); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"drop", "type", "downgrade", "missing resource"} {
		t.Run(mode, func(t *testing.T) {
			next := expanded.Clone()
			layouts := map[string]schema.MessageLayout{"events": next}
			switch mode {
			case "drop":
				next.Fields = next.Fields[:1]
				layouts["events"] = next
			case "type":
				next.Fields[0].Logical = rowmodel.LogicalJSON
			case "missing resource":
				delete(layouts, "events")
				layouts["other"] = layout
			}
			r := request(3, "3", layouts)
			if mode == "downgrade" {
				r = f.epoch(t, a, 3, "3")
			}
			_, err := f.store.CommitEpoch(ctx, r)
			if !errors.Is(err, schema.ErrBlocked) && !errors.Is(err, filament.ErrEpochConflict) && !errors.Is(err, filament.ErrMembershipChanged) {
				t.Fatalf("unsafe layout admitted: %v", err)
			}
		})
	}
	s, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || s.LastEpoch.Epoch != 2 {
		t.Fatalf("rejected layouts advanced progress: %+v %v", s, err)
	}
}
