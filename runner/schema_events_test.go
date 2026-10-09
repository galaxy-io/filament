package runner

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/schema"
)

func TestSchemaEventsPublishWithoutAdvancingProgress(t *testing.T) {
	bus := inproc.New()
	t.Cleanup(func() { _ = bus.Close() })
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	cfg := ContinuousConfig{Bus: bus}
	cfg.Spec.Tenant, cfg.Spec.Run = "tenant", "run"
	finish := startContinuousEvents(t.Context(), &cfg)
	ctx := observeSchemaEvents(t.Context(), cfg.events)
	schema.Observe(ctx, "sink", "users", schema.NullableAdd, schema.Diff{})
	diff := schema.Diff{Changes: []schema.Change{{Kind: schema.AddField, Field: "email"}}}
	schema.Observe(ctx, "sink", "users", schema.NullableAdd, diff)
	finish(errors.Join(errors.New("cleanup"), &schema.BlockError{Resource: "users", Reasons: []string{"required field"}}))
	for i, name := range []string{events.SchemaDetected.Name(), events.SchemaBlocked.Name(), events.StreamAttemptEnded.Name()} {
		select {
		case msg := <-sub.C():
			fact, err := events.Decode(msg)
			if err != nil {
				t.Fatal(err)
			}
			if fact.Name != name || fact.Tenant != "tenant" || fact.Run != "run" || fact.Seq != uint64(i+1) {
				t.Fatalf("incorrect fact: %+v", fact)
			}
			if i < 2 && fact.Resource != "users" {
				t.Fatal("lost resource")
			}
			raw, err := events.Marshal(fact)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := events.Unmarshal(raw)
			if err != nil || !reflect.DeepEqual(decoded.Data, fact.Data) {
				t.Fatalf("wire roundtrip: %+v %v", decoded, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %s", name)
		}
	}
	if cfg.events.runRecords != 0 || cfg.events.runBytes != 0 || len(cfg.events.res) != 0 {
		t.Fatal("diagnostics advanced progress")
	}
}
