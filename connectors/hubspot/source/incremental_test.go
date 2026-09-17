package hubspot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
)

type configuredFixtureSource struct {
	*Source
	url   string
	upper time.Time
}

func (s *configuredFixtureSource) Configure(ctx context.Context, config filament.Config) error {
	if err := s.Source.Configure(ctx, config); err != nil {
		return err
	}
	s.client.baseURL = s.url
	s.client.limiter = rate.NewLimiter(rate.Inf, 1)
	s.now = func() time.Time { return s.upper }
	return nil
}

// Exercises the real runner, Arrow pipeline, receipts, event bus, and tracker.
// The terminal result is delivered only after the tracker has folded it.
func runFixture(t *testing.T, store filament.DataStore, fixture *objectFixture, name, run string, sink *captureSink, pause bool, upper time.Time) (events.Fact, time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facts.Close() }()
	folder := tracker.New()
	if err := folder.Mount(ctx, module.Deps{DataStore: store}); err != nil {
		t.Fatal(err)
	}
	done := make(chan events.Fact, 1)
	failures := make(chan error, 1)
	handler := folder.Subscriptions()[0].Handler
	go func() {
		for msg := range facts.C() {
			if err := handler(ctx, msg); err != nil {
				failures <- err
				return
			}
			fact, err := events.Decode(msg)
			_ = msg.Ack()
			if err != nil {
				failures <- err
				return
			}
			switch fact.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent, events.RunPausedEvent, events.RunCanceledEvent:
				done <- fact
			}
		}
	}()
	spec := filament.RunSpec{
		Tenant: "tenant", Run: filament.RunID(run), PipelineID: "pipe", PipelineVersionID: "v1", CheckpointRoute: "hubspot/capture",
		Source: filament.Ref{Connector: "hubspot", Config: map[string]any{"api_key": "test"}}, Sink: filament.Ref{Connector: "capture"},
		Resources: []string{name}, IngestionTypes: map[string]filament.IngestionType{name: filament.IngestionIncrementalUpsert},
		Options: filament.RunOptions{BatchMaxRows: 10, SnapshotParallelism: 3, CheckpointEvery: 1},
	}
	request := filament.RunRequest{Tenant: spec.Tenant, PipelineID: spec.PipelineID, PipelineVersionID: spec.PipelineVersionID, CheckpointRoute: spec.CheckpointRoute, Resources: spec.Resources, IngestionTypes: spec.IngestionTypes}
	if err := store.SaveRun(ctx, filament.RunState{Run: spec.Run, Tenant: spec.Tenant, Status: filament.RunRequested, Request: request}); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.Contains(req.URL.Path, "/properties/") && !strings.HasSuffix(req.URL.Path, "/batch/read") {
			requests++
			if pause && requests == 2 {
				_, _ = io.Copy(io.Discard, req.Body)
				fact := events.NewFact(events.RunPauseRequested, events.Envelope{Tenant: spec.Tenant, Run: spec.Run}, events.RunPauseRequestedEvent{})
				if err := bus.Publish(ctx, events.Subject(events.RunPauseRequested, spec.Tenant, spec.Run), fact); err != nil {
					t.Error(err)
				}
				<-req.Context().Done()
				return
			}
		}
		fixture.ServeHTTP(w, req)
	}))
	defer server.Close()
	sources := registry.NewSources()
	sources.Register("hubspot", func() filament.Source { return &configuredFixtureSource{Source: New(), url: server.URL, upper: upper} })
	sinks := registry.NewSinks()
	sinks.Register("capture", func() filament.Sink { return sink })
	if err := runner.RunOne(ctx, runner.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks}, spec); err != nil {
		t.Fatal(err)
	}
	var terminal events.Fact
	select {
	case terminal = <-done:
	case err := <-failures:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("runner/tracker timed out")
	}
	key, _ := spec.ResourceCheckpointKey(name)
	state, err := store.LoadResourceCheckpoint(ctx, spec.Tenant, key)
	if err != nil {
		t.Fatal(err)
	}
	mark, err := planWatermark(name, state.Checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	return terminal, mark
}

func TestRunnerCommitsAndLoadsIncrementalWatermark(t *testing.T) {
	for _, name := range []string{"contacts", "custom_objects_2_123"} {
		t.Run(name, func(t *testing.T) {
			store := sqlite.NewMemory()
			defer func() { _ = store.Close() }()
			r, _ := lookupResource(name)
			fixture := &objectFixture{t: t, count: 250, modified: r.modified}
			terminal, mark := runFixture(t, store, fixture, name, "bootstrap", &captureSink{}, false, testTime)
			if _, ok := terminal.Data.(events.RunCompletedEvent); !ok || !mark.Equal(testTime) {
				t.Fatalf("bootstrap: %T checkpoint=%v", terminal.Data, mark)
			}
			fixture = &objectFixture{t: t, count: 10, modified: r.modified, selected: testTime.Add(time.Minute)}
			terminal, mark = runFixture(t, store, fixture, name, "incremental", &captureSink{}, false, testTime.Add(time.Hour))
			if _, ok := terminal.Data.(events.RunCompletedEvent); !ok || !mark.Equal(testTime.Add(time.Minute)) {
				t.Fatalf("incremental: %T checkpoint=%v", terminal.Data, mark)
			}
			if fixture.lists != 0 || len(fixture.searches) != 2 {
				t.Fatalf("lists=%d searches=%d", fixture.lists, len(fixture.searches))
			}
		})
	}
}

func TestRunnerPreservesWatermarkOnPauseOrFailure(t *testing.T) {
	for _, test := range []struct {
		name                    string
		pause, bootstrap        bool
		applyError, commitError error
	}{
		{name: "committed pause", pause: true},
		{name: "bootstrap pause", pause: true, bootstrap: true},
		{name: "sink write failure", applyError: errors.New("write failed")},
		{name: "sink commit failure", commitError: errors.New("commit failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := sqlite.NewMemory()
			defer func() { _ = store.Close() }()
			previous := testTime.Add(-time.Hour)
			if test.bootstrap {
				previous = initialWatermark
			}
			request := filament.RunRequest{PipelineID: "pipe", PipelineVersionID: "v1", CheckpointRoute: "hubspot/capture", IngestionTypes: map[string]filament.IngestionType{"contacts": filament.IngestionIncrementalUpsert}}
			key, _ := request.ResourceCheckpointKey("contacts")
			if err := store.SaveResourceCheckpoint(t.Context(), "tenant", filament.ResourceCheckpointState{Key: key, Run: "previous", Checkpoint: watermarkPlan("contacts", previous)}); err != nil {
				t.Fatal(err)
			}
			fixture := &objectFixture{t: t, count: 250, modified: "lastmodifieddate"}
			sink := &captureSink{applyError: test.applyError, commitError: test.commitError}
			terminal, mark := runFixture(t, store, fixture, "contacts", "current", sink, test.pause, testTime)
			if !mark.Equal(previous) {
				t.Fatalf("%T advanced checkpoint from %v to %v", terminal.Data, previous, mark)
			}
			if test.pause {
				paused, ok := terminal.Data.(events.RunPausedEvent)
				if !ok || !paused.Committed {
					t.Fatalf("expected committed pause, got %+v", terminal.Data)
				}
			} else if _, completed := terminal.Data.(events.RunCompletedEvent); completed {
				t.Fatal("failed sink completed run")
			}
		})
	}
}
