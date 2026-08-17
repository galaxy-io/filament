package filament

import "testing"

func TestStandardSyncModeIngestionType(t *testing.T) {
	tests := []struct {
		name string
		mode StandardSyncMode
		want IngestionType
	}{
		{"unspecified defaults to replace", "", IngestionFullReplace},
		{"replace", StandardSyncReplace, IngestionFullReplace},
		{"append", StandardSyncAppend, IngestionFullAppend},
		{"incremental", StandardSyncIncremental, IngestionIncrementalUpsert},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mode.IngestionType()
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
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
