package filament

import "testing"

func TestIngestionFor(t *testing.T) {
	tests := []struct {
		name  string
		read  ReadMode
		write WriteMode
		want  IngestionType
		fails bool
	}{
		{"defaults to full refresh", ModeFull, "", IngestionFullReplace, false},
		{"incremental defaults to upsert", ModeIncremental, "", IngestionIncrementalUpsert, false},
		{"full replace", ModeFull, WriteReplace, IngestionFullReplace, false},
		{"full upsert", ModeFull, WriteUpsert, IngestionFullUpsert, false},
		{"full append", ModeFull, WriteAppend, IngestionFullAppend, false},
		{"incremental append", ModeIncremental, WriteAppend, IngestionIncrementalAppend, false},
		{"incremental upsert", ModeIncremental, WriteUpsert, IngestionIncrementalUpsert, false},
		{"incremental delete", ModeIncremental, WriteDelete, IngestionIncrementalDelete, false},
		{"incremental replace is incoherent", ModeIncremental, WriteReplace, "", true},
		{"full delete is incoherent", ModeFull, WriteDelete, "", true},
		{"merge is never a lever", ModeFull, WriteMerge, "", true},
		{"cdc never compiles from levers", ModeCDC, WriteMerge, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IngestionFor(tt.read, tt.write)
			if tt.fails {
				if err == nil {
					t.Fatalf("want error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
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
