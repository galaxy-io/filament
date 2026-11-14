package iceberg

import (
	"testing"
	"time"

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
