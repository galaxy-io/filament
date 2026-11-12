package object

import (
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

func TestContinuousLayoutPreservesRunAndRefreshesDate(t *testing.T) {
	partition, err := ParsePartition("{{.Run}}/dt={{.Date}}")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 1, 1, 23, 59, 0, 0, time.UTC)
	first := filament.RunSpec{Run: "real-run", StartedAt: started, SinkWriteID: "write-a", SinkWriteStartedAt: started}
	second := first
	second.SinkWriteID = "write-b"
	second.SinkWriteStartedAt = started.Add(2 * time.Minute)
	for _, tc := range []struct {
		run  filament.RunSpec
		date string
	}{{first, "2026-01-01"}, {second, "2026-01-02"}} {
		l := NewLayout("exports", partition, ".json", tc.run)
		key, err := l.Object("items")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(key, "real-run/dt="+tc.date+"/"+string(tc.run.SinkWriteID)) {
			t.Fatalf("wrong object identity or partition: %s", key)
		}
		if l.Run() != "real-run" || !strings.Contains(l.Success(), "real-run/"+string(tc.run.SinkWriteID)+"/") {
			t.Fatalf("wrong manifest identity: %+v", l)
		}
	}
}
