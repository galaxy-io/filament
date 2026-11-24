package iceberg

import (
	"context"
	"slices"
	"testing"
	"time"

	icetable "github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
)

func TestEpochBoundaryFollowsCommitInterval(t *testing.T) {
	s := New()
	if got := *s.Spec().Capabilities.PreferredEpochBoundary; got.MaxWait != time.Minute || got.MaxAge != time.Minute {
		t.Fatalf("preferred = %+v", got)
	}
	for _, tc := range []struct {
		cfg  map[string]any
		want time.Duration
	}{
		{cfg: nil, want: time.Minute},
		{cfg: map[string]any{"commit_interval": nil}, want: time.Minute},
		{cfg: map[string]any{"commit_interval": ""}, want: time.Minute},
		{cfg: map[string]any{"commit_interval": "5m"}, want: 5 * time.Minute},
	} {
		got, err := s.EpochBoundary(filament.NewConfig(tc.cfg))
		if err != nil {
			t.Fatal(err)
		}
		if got.MaxWait != tc.want || got.MaxAge != tc.want || got.MaxRecords != 1_000_000 || got.MaxBytes != defaultStageBufLimitBytes {
			t.Fatalf("%v: boundary = %+v", tc.cfg, got)
		}
	}
	got, err := s.EpochBoundary(filament.NewConfig(map[string]any{"stage_buffer_limit_mb": 64}))
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxBytes != 64<<20 {
		t.Fatalf("MaxBytes = %d, want stage_buffer_limit_mb", got.MaxBytes)
	}
	for _, bad := range []any{"0s", "-1m", "soon"} {
		if _, err := s.EpochBoundary(filament.NewConfig(map[string]any{"commit_interval": bad})); err == nil {
			t.Fatalf("commit_interval %q accepted", bad)
		}
	}
}

func TestCommitIntervalDefaultMatches(t *testing.T) {
	for _, f := range New().Spec().Config.Fields {
		if f.Name != "commit_interval" {
			continue
		}
		if d, err := time.ParseDuration(f.Default.(string)); err != nil || d != defaultCommitInterval {
			t.Fatalf("commit_interval default %v does not match %v", f.Default, defaultCommitInterval)
		}
		return
	}
	t.Fatal("commit_interval field missing")
}

// TestUpdateModePerTable checks that a table's own update mode wins over the
// pipeline's, that copy-on-write is the default, and that an unknown mode
// fails the run before any table is created.
func TestUpdateModePerTable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	open := func(configs map[string]map[string]any) (*Sink, error) {
		sink := New()
		return sink, sink.Open(ctx, filament.RunSpec{
			Run: "test", StartedAt: time.Now(), Resources: []string{"plain", "merged"},
			Sink:        filament.Ref{Connector: "iceberg", Config: env.cfg},
			SinkConfigs: configs,
		})
	}
	sink, err := open(map[string]map[string]any{"merged": {"update_mode": icetable.WriteModeMergeOnRead}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sink.Close(ctx) }()
	for resource, want := range map[string]string{"plain": icetable.WriteModeCopyOnWrite, "merged": icetable.WriteModeMergeOnRead} {
		if err := sink.EnsureSchema(ctx, resource, env.schema); err != nil {
			t.Fatal(err)
		}
		props := sink.tables[resource].tbl.Properties()
		if props[writeMergeModeKey] != want || props[icetable.WriteDeleteModeKey] != want {
			t.Fatalf("%s: merge mode %q, delete mode %q, want %q", resource, props[writeMergeModeKey], props[icetable.WriteDeleteModeKey], want)
		}
	}
	if _, err := open(map[string]map[string]any{"plain": {"update_mode": "eventually"}}); err == nil {
		t.Fatal("unknown table update mode accepted")
	}
	env.updateMode("eventually")
	if _, err := open(nil); err == nil {
		t.Fatal("unknown pipeline update mode accepted")
	}
}

// TestPartitionByPerTable checks that a table's partition_by becomes its
// partition spec at creation, that the sink can write to it, and that a bad
// entry fails the run.
func TestPartitionByPerTable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	open := func(partitionBy ...any) (*Sink, error) {
		sink := New()
		policy := eventsPolicy(filament.IngestionFullAppend)
		return sink, sink.Open(ctx, filament.RunSpec{
			Run: "test", StartedAt: time.Now(), Resources: []string{"events"},
			Sink:          filament.Ref{Connector: "iceberg", Config: env.cfg},
			WritePolicies: map[string]filament.WritePolicy{"events": policy},
			SinkConfigs:   map[string]map[string]any{"events": {"partition_by": partitionBy}},
		})
	}
	for _, bad := range []string{"fortnight(id)", "day(id", "(id)", ""} {
		if _, err := open(bad); err == nil {
			t.Fatalf("partition_by %q accepted", bad)
		}
	}
	sink, err := open("missing")
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.EnsureSchema(ctx, "events", env.schema); err == nil {
		t.Fatal("partition_by on an unknown column accepted")
	}
	_ = sink.Close(ctx)

	sink, err = open("bucket[4](id)", "note")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sink.Close(ctx) }()
	if err := sink.EnsureSchema(ctx, "events", env.schema); err != nil {
		t.Fatal(err)
	}
	var names []string
	spec := sink.tables["events"].tbl.Spec()
	for _, field := range spec.Fields() {
		names = append(names, field.Name+" "+field.Transform.String())
	}
	if want := []string{"id_bucket bucket[4]", "note identity"}; !slices.Equal(names, want) {
		t.Fatalf("partition fields = %v, want %v", names, want)
	}
	if err := sink.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	batch := idBatch(t, 0, 20)
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: eventsPolicy(filament.IngestionFullAppend)}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, env, idRange(0, 20))
}
