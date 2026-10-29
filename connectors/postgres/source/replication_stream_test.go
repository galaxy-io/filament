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

func TestContinuousAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, replication, snapshot string
		resources                   []string
		valid                       bool
	}{
		{"changes", "cdc", "none", []string{"orders", "customers"}, true},
		{"initial", "cdc", "initial", []string{"orders"}, true},
		{"default snapshot", "cdc", "", []string{"orders"}, true},
		{"standard", "standard", "none", []string{"orders"}, false},
		{"missing tables", "cdc", "none", nil, false},
		{"duplicate table", "cdc", "none", []string{"orders", "orders"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := filament.ReplicationStreamPlanningRequest{Execution: filament.ExecutionContinuous, ReplicationStreamID: "stream", Resources: tc.resources, Config: filament.NewConfig(map[string]any{"replication": tc.replication, "snapshot_mode": tc.snapshot})}
			plan, err := New().PlanReplicationStream(req)
			if (err == nil) != tc.valid {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if tc.valid {
				if !reflect.DeepEqual(plan.Resources, tc.resources) {
					t.Fatal("membership lost")
				}
				plan.Resources[0] = "changed"
				if tc.resources[0] == "changed" {
					t.Fatal("membership aliases caller slice")
				}
			}
		})
	}
}
