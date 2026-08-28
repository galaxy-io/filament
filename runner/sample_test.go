package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sample"
	"github.com/galaxy-io/filament/connectors/stdout"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
)

// TestRunSampleToStdout drives the whole loop end to end: the sample source
// appends rows into builders, the pipeline batches and verifies them, and the
// stdout sink renders them as NDJSON.
func TestRunSampleToStdout(t *testing.T) {
	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()

	completed := make(chan events.RunCompletedEvent, 1)
	go func() {
		for msg := range facts.C() {
			f, err := events.Decode(msg)
			_ = msg.Ack()
			if err != nil {
				continue
			}
			if d, ok := f.Data.(events.RunCompletedEvent); ok {
				completed <- d
			}
		}
	}()

	var out bytes.Buffer
	sources := registry.NewSources()
	sources.Register("sample", func() filament.Source { return sample.New() })
	sinks := registry.NewSinks()
	sinks.Register("stdout", func() filament.Sink { return stdout.New(stdout.WithWriter(&out)) })

	RunOne(context.Background(), Deps{
		Bus:       bus,
		DataStore: memory.New(),
		Sources:   sources,
		Sinks:     sinks,
	}, filament.RunSpec{
		Tenant:         "t1",
		Run:            "r1",
		Source:         filament.Ref{Connector: "sample", Config: map[string]any{"rows": 5}},
		Sink:           filament.Ref{Connector: "stdout"},
		Resources:      []string{"users", "orders"},
		Options:        filament.RunOptions{BatchMaxRows: 2},
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullReplace},
	})

	select {
	case d := <-completed:
		if d.Records != 10 {
			t.Fatalf("run.completed records = %d, want 10", d.Records)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no run.completed fact published")
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var data []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "#") {
			data = append(data, l)
		}
	}
	if len(data) != 10 {
		t.Fatalf("data lines = %d, want 10:\n%s", len(data), out.String())
	}
	if !strings.Contains(out.String(), `{"i":4,"resource":"orders","_filament_run_id":"r1"`) {
		t.Fatalf("missing last orders row:\n%s", out.String())
	}
	if !strings.HasPrefix(lines[len(lines)-1], "# {") {
		t.Fatalf("manifest missing:\n%s", out.String())
	}
}
