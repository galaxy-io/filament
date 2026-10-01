package filament

import (
	"context"
	"errors"
	"testing"
)

func TestIngestionFor(t *testing.T) {
	tests := []struct {
		name  string
		read  ReadMode
		write WriteMode
		want  IngestionType
	}{
		{"defaults to full upsert", ModeFull, "", IngestionFullUpsert},
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
	if err := ValidateReplication(ReplicationCDC, IngestionCDCMerge); err != nil {
		t.Fatal(err)
	}
	if err := ValidateReplication(ReplicationStandard, IngestionCDCMerge); err == nil {
		t.Fatal("cdc on a standard connection must fail")
	}
	if err := ValidateReplication(ReplicationCDC, IngestionFullReplace); err == nil {
		t.Fatal("levers on a cdc connection must fail")
	}
}

func TestCDCAppendPolicy(t *testing.T) {
	policy := WritePolicyForIngestion(IngestionCDCAppend)
	if policy.Capability.Mode != WriteAppend {
		t.Fatalf("mode = %q, want append", policy.Capability.Mode)
	}
	if !policy.Capability.RequiresOrder || !policy.Capability.RequiresPK {
		t.Fatalf("CDC append capability = %+v", policy.Capability)
	}
	want := []Operation{OpInsert, OpUpdate, OpDelete}
	if !acceptsOperations(policy.Capability.AcceptsOps, want) {
		t.Fatalf("accepted operations = %v, want %v", policy.Capability.AcceptsOps, want)
	}
	if policy.Checkpoint != CheckpointAfterCommit {
		t.Fatalf("checkpoint = %q, want after commit", policy.Checkpoint)
	}
}

func TestCheckpointCoverageFor(t *testing.T) {
	resources := []string{"users", "audit"}
	if got := CheckpointCoverageFor(resources, map[string]IngestionType{
		"users": IngestionFullUpsert,
		"audit": IngestionFullAppend,
	}); got != CheckpointCoverageSome {
		t.Fatalf("mixed coverage = %v, want some", got)
	}
	if got := CheckpointCoverageFor(resources, map[string]IngestionType{
		"users": IngestionFullUpsert,
		"audit": IngestionIncrementalUpsert,
	}); got != CheckpointCoverageAll {
		t.Fatalf("resumable coverage = %v, want all", got)
	}
	if got := CheckpointCoverageFor(resources, nil); got != CheckpointCoverageNone {
		t.Fatalf("default coverage = %v, want none", got)
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

func TestUndeclaredWritePoliciesRejected(t *testing.T) {
	appendOnly := SinkSpec{Name: "append-only", Capabilities: SinkCapabilities{WritePolicies: WriteCapabilities(IngestionFullAppend)}}
	for _, mode := range []IngestionType{IngestionFullReplace, IngestionFullUpsert, IngestionCDCAppend} {
		if err := ValidateSinkIngestion(appendOnly, mode); err == nil {
			t.Fatalf("append-only sink accepted %s", mode)
		}
	}
	if err := ValidateSinkIngestion(appendOnly, IngestionFullAppend); err != nil {
		t.Fatal(err)
	}
	bare := SinkSpec{Name: "bare"}
	for _, mode := range []IngestionType{IngestionFullReplace, IngestionFullAppend, IngestionFullUpsert} {
		if err := ValidateSinkIngestion(bare, mode); err == nil {
			t.Fatalf("sink without write policies accepted %s", mode)
		}
	}
}

func TestBindSinkRecordAtomicity(t *testing.T) {
	policy := WritePolicyForIngestion(IngestionFullAppend)
	bindSinkDurability(&policy, WritePolicyCapability{Atomicity: AtomicityRecord, Durability: DurabilityAfterApply})
	if policy.Capability.Atomicity != AtomicityRecord {
		t.Fatal("planner retained batch atomicity for record-at-a-time sink")
	}
}

type mixedKeySource struct {
	Source
	schemaErr error
}

func (s mixedKeySource) Spec() ConnectorSpec {
	return ConnectorSpec{SourcePolicies: SourcePolicies(IngestionFullUpsert, IngestionIncrementalUpsert, IngestionCDCMerge)}
}

func (s mixedKeySource) Schema(_ context.Context, resource string) (RecordSchema, error) {
	if s.schemaErr != nil {
		return RecordSchema{}, s.schemaErr
	}
	if resource == "keyed" {
		return RecordSchema{PrimaryKey: []string{"id"}}, nil
	}
	return RecordSchema{}, nil
}

type mixedKeySink struct {
	Sink
	types []IngestionType
}

func (s mixedKeySink) Spec() SinkSpec {
	return SinkSpec{Capabilities: SinkCapabilities{WritePolicies: WriteCapabilities(s.types...)}}
}

func TestResolveIngestionPlanKeylessUpsertFallback(t *testing.T) {
	for _, ingestion := range []IngestionType{IngestionFullUpsert, IngestionIncrementalUpsert} {
		t.Run(string(ingestion), func(t *testing.T) {
			fallback := IngestionFullAppend
			if ingestion == IngestionIncrementalUpsert {
				fallback = IngestionIncrementalAppend
			}
			spec := RunSpec{Resources: []string{"keyed", "keyless"}, IngestionTypes: map[string]IngestionType{"": ingestion}, WritePolicies: map[string]WritePolicy{"keyless": {DestinationResource: "events"}}}
			plan, err := ResolveIngestionPlan(context.Background(), mixedKeySource{}, mixedKeySink{types: []IngestionType{ingestion, fallback}}, spec)
			if err != nil {
				t.Fatal(err)
			}
			keyed, keyless := plan.WritePolicies["keyed"], plan.WritePolicies["keyless"]
			if keyed.Capability.Mode != WriteUpsert || len(keyed.Keys) != 1 {
				t.Fatalf("keyed policy: %+v", keyed)
			}
			if keyless.Capability.Mode != WriteAppend || keyless.Capability.RequiresPK || len(keyless.Keys) != 0 || keyless.DestinationResource != "events" {
				t.Fatalf("keyless policy: %+v", keyless)
			}
			if plan.IngestionTypes["keyed"] != ingestion || plan.IngestionTypes["keyless"] != fallback {
				t.Fatalf("effective types: %v", plan.IngestionTypes)
			}
			if keyless.Checkpoint != WritePolicyForIngestion(fallback).Checkpoint {
				t.Fatalf("wrong checkpoint: %s", keyless.Checkpoint)
			}
			if _, ok := spec.IngestionTypes["keyless"]; ok {
				t.Fatal("input plan mutated")
			}
			if _, err := ResolveIngestionPlan(context.Background(), mixedKeySource{}, mixedKeySink{types: []IngestionType{ingestion}}, spec); err == nil {
				t.Fatal("missing append capability accepted")
			}
		})
	}
}

func TestIngestionForKeysPreservesExplicitModes(t *testing.T) {
	for _, mode := range []IngestionType{IngestionFullReplace, IngestionFullAppend, IngestionIncrementalAppend, IngestionCDCMerge, IngestionCDCAppend, IngestionIncrementalDelete} {
		if got := IngestionForKeys(mode, nil); got != mode {
			t.Fatalf("%s changed to %s", mode, got)
		}
	}
}

func TestKeylessFallbackDoesNotHideSchemaErrors(t *testing.T) {
	expected := errors.New("schema unavailable")
	_, err := ResolveIngestionPlan(context.Background(), mixedKeySource{schemaErr: expected}, mixedKeySink{types: []IngestionType{IngestionFullUpsert, IngestionFullAppend}}, RunSpec{Resources: []string{"keyless"}, IngestionTypes: map[string]IngestionType{"": IngestionFullUpsert}})
	if !errors.Is(err, expected) {
		t.Fatalf("schema error was hidden: %v", err)
	}
}
