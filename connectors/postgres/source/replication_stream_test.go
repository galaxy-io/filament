package postgres

import (
	"reflect"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestReplicationStreamPlannerOwnsPostgresSlotConfiguration(t *testing.T) {
	source := New()
	for _, field := range source.Spec().Config.Fields {
		if field.Name == "slot_name" {
			t.Fatal("slot_name must remain internal runtime configuration")
		}
	}
	first, err := source.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{
		ReplicationStreamID: "60000000-0000-4000-8000-000000000001",
		Config: filament.NewConfig(map[string]any{
			"schema": "public", "publication": "shared", "page_size": 1000,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{
		ReplicationStreamID: "60000000-0000-4000-8000-000000000001",
		Config: filament.NewConfig(map[string]any{
			"schema": "public", "publication": "shared", "page_size": 5000,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ConsumerName != "filament_60000000000040008000000000000001" {
		t.Fatalf("consumer name = %q", first.ConsumerName)
	}
	if !reflect.DeepEqual(first.ContinuityConfig, second.ContinuityConfig) {
		t.Fatalf("read tuning changed continuity: %#v != %#v", first.ContinuityConfig, second.ContinuityConfig)
	}

	config := map[string]any{"schema": "public"}
	bound, err := source.BindReplicationStream(config, filament.ReplicationStream{ConsumerName: first.ConsumerName})
	if err != nil {
		t.Fatal(err)
	}
	if _, mutated := config["slot_name"]; mutated || bound["slot_name"] != first.ConsumerName {
		t.Fatalf("binding mutated input or missed slot: input=%#v bound=%#v", config, bound)
	}
}
