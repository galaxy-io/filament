package filament

import (
	"context"
	"testing"
)

func TestIngestionFor(t *testing.T) {
	tests := []struct {
		name  string
		read  ReadMode
		write WriteMode
		want  IngestionType
	}{
		{"defaults to full replace", ModeFull, "", IngestionFullReplace},
		{"full replace", ModeFull, WriteReplace, IngestionFullReplace},
		{"full append", ModeFull, WriteAppend, IngestionFullAppend},
		{"full upsert", ModeFull, WriteUpsert, IngestionFullUpsert},
		{"incremental defaults to upsert", ModeIncremental, "", IngestionIncrementalUpsert},
		{"incremental append", ModeIncremental, WriteAppend, IngestionIncrementalAppend},
		{"incremental upsert", ModeIncremental, WriteUpsert, IngestionIncrementalUpsert},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IngestionFor(tt.read, tt.write)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIngestionForRejectsIncrementalReplace(t *testing.T) {
	if _, err := IngestionFor(ModeIncremental, WriteReplace); err == nil {
		t.Fatal("incremental replace must be rejected")
	}
}

func TestValidateReplication(t *testing.T) {
	if err := ValidateReplication(ReplicationStandard, IngestionIncrementalUpsert); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplication(ReplicationCDC, IngestionCDC); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplication(ReplicationStandard, IngestionCDC); err == nil {
		t.Fatal("cdc on a standard connection must fail")
	}
	if err := ValidateReplication(ReplicationCDC, IngestionFullReplace); err == nil {
		t.Fatal("levers on a cdc connection must fail")
	}
}

type replicationSource struct{ Source }

func (replicationSource) Replication(cfg Config) ReplicationMode {
	if cfg.String("replication") == string(ReplicationCDC) {
		return ReplicationCDC
	}
	return ReplicationStandard
}

func TestReplicationOf(t *testing.T) {
	aware := replicationSource{}
	if got := ReplicationOf(aware, NewConfig(map[string]any{"replication": "cdc"})); got != ReplicationCDC {
		t.Fatalf("got %q, want cdc", got)
	}
	if got := ReplicationOf(aware, NewConfig(nil)); got != ReplicationStandard {
		t.Fatalf("got %q, want standard", got)
	}
	var unaware Source
	if got := ReplicationOf(unaware, NewConfig(nil)); got != ReplicationStandard {
		t.Fatalf("unaware source: got %q, want standard", got)
	}
}

type durabilityTestSource struct{ Source }

func (durabilityTestSource) Spec() ConnectorSpec {
	return ConnectorSpec{SourcePolicies: []SourcePolicy{SourcePolicyForIngestion(IngestionFullUpsert)}}
}

func (durabilityTestSource) Schema(context.Context, string) (RecordSchema, error) {
	return RecordSchema{PrimaryKey: []string{"id"}}, nil
}

type durabilityTestSink struct {
	Sink
	durability WriteDurability
}

func (s durabilityTestSink) Spec() SinkSpec {
	capability := WritePolicyForIngestion(IngestionFullUpsert).Capability
	capability.Durability = s.durability
	return SinkSpec{Name: "test", Capabilities: SinkCapabilities{WritePolicies: []WritePolicyCapability{capability}}}
}

func TestResolveIngestionPlanUsesSinkDurabilityBoundary(t *testing.T) {
	spec := RunSpec{
		Resources:      []string{"users"},
		IngestionTypes: map[string]IngestionType{"users": IngestionFullUpsert},
	}
	for _, tt := range []struct {
		name       string
		durability WriteDurability
		want       CheckpointPolicy
	}{
		{name: "apply durable", durability: DurabilityAfterApply, want: CheckpointAfterBatch},
		{name: "commit durable", durability: DurabilityAfterCommit, want: CheckpointAfterCommit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := ResolveIngestionPlan(context.Background(), durabilityTestSource{}, durabilityTestSink{durability: tt.durability}, spec)
			if err != nil {
				t.Fatal(err)
			}
			if got := plan.WritePolicies["users"].Checkpoint; got != tt.want {
				t.Fatalf("checkpoint policy = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveIngestionPlanRejectsMissingSinkDurability(t *testing.T) {
	spec := RunSpec{
		Resources:      []string{"users"},
		IngestionTypes: map[string]IngestionType{"users": IngestionFullUpsert},
	}
	_, err := ResolveIngestionPlan(context.Background(), durabilityTestSource{}, durabilityTestSink{}, spec)
	if err == nil {
		t.Fatal("sink capability without durability was accepted")
	}
}
