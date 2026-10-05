package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

func TestExpiredScopedReplayAndProgress(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `[{"id":"row"}]`) }))
	t.Cleanup(api.Close)
	for _, phase := range []string{"planned", "waiting", "done", "creating"} {
		t.Run(phase, func(t *testing.T) {
			control := &recoveryControl{url: api.URL + "?signature=sensitive"}
			var progress []filament.SourceProgress
			c := &Runtime{Control: control, Transport: api.Client().Transport, Observe: func(p filament.SourceProgress) { progress = append(progress, p) }}
			res := recoveryResource()
			state := recoveryState(c, res, false, time.Now().Add(-time.Hour), phase)
			var err error
			c.ResumeToken, err = state.token()
			if err != nil {
				t.Fatal(err)
			}
			sink := &snapshotSink{}
			err = c.Run(t.Context(), res, sink, nil)
			if phase == "creating" {
				if err == nil || !strings.Contains(err.Error(), "reconcile") || control.starts != 0 || control.polls != 0 || len(progress) != 0 {
					t.Fatalf("ambiguous job retried: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if control.polls != 1 || sink.rows != 1 {
				t.Fatal("expired job was not replayed")
			}
			want := []filament.SourceProgressKind{filament.SourceProgressExportJobPolled, filament.SourceProgressExportJobReady}
			if phase == "planned" {
				want = append([]filament.SourceProgressKind{filament.SourceProgressExportJobCreated}, want...)
				if control.starts != 1 {
					t.Fatal("planned job not created")
				}
			} else if control.starts != 0 {
				t.Fatal("captured job recreated")
			}
			if len(progress) != len(want) {
				t.Fatalf("progress = %#v", progress)
			}
			for i, p := range progress {
				if p.Kind != want[i] || p.Resource != res.Name || p.ExportJob.CorrelationID == "" || p.ExportJob.CorrelationID != progress[0].ExportJob.CorrelationID || p.ExportJob.Resumed != (phase != "planned") {
					t.Fatalf("unexpected event: %#v", p)
				}
			}
			raw, _ := json.Marshal(progress)
			if strings.Contains(string(raw), "sensitive") || strings.Contains(string(raw), api.URL) {
				t.Fatal("sensitive job metadata leaked")
			}
			var final RunState
			if err := json.Unmarshal([]byte(sink.tokens[len(sink.tokens)-1]), &final); err != nil {
				t.Fatal(err)
			}
			if final.Parents["0"].Job.Phase != "done" || final.Parents["0"].Committed != "committed" {
				t.Fatal("final snapshot lost completion or advanced watermark")
			}
		})
	}
}

func TestJobProgressPollSampling(t *testing.T) {
	var reports []filament.SourceProgress
	c := &Runtime{Observe: func(p filament.SourceProgress) { reports = append(reports, p) }}
	p := c.jobProgress("items", false)
	for range 100 {
		p.polled()
	}
	if len(reports) != 1 {
		t.Fatal("polls were not sampled")
	}
	p.lastPoll = time.Now().Add(-exportPollReportInterval)
	p.polled()
	p.ready()
	p.ready() // An expired artifact URL may cause another readiness check.
	if len(reports) != 3 || reports[1].ExportJob.Polls != 101 || reports[2].Kind != filament.SourceProgressExportJobReady || reports[2].ExportJob.Polls != 101 {
		t.Fatalf("sampled progress = %#v", reports)
	}
}

func TestExportEventsRequireValidatedLifecycleTransitions(t *testing.T) {
	for _, tc := range []struct {
		name, start, poll string
		created, polled   bool
	}{
		{"invalid_creation", `{}`, ``, false, false},
		{"invalid_ready_captures", ``, `{"status":"ready"}`, true, true},
		{"invalid_status", ``, `{"status":"unexpected"}`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := &recoveryControl{startResponse: tc.start, pollResponse: tc.poll}
			var reports []filament.SourceProgressKind
			c := &Runtime{Control: control, Observe: func(p filament.SourceProgress) { reports = append(reports, p.Kind) }}
			res := recoveryResource()
			res.Parent = nil // Exercise the top-level lifecycle as well as scoped recovery.
			if err := c.Run(t.Context(), res, &snapshotSink{}, nil); err == nil {
				t.Fatal("invalid lifecycle succeeded")
			}
			created, polled := false, false
			for _, kind := range reports {
				switch kind {
				case filament.SourceProgressExportJobCreated:
					created = true
				case filament.SourceProgressExportJobPolled:
					polled = true
				case filament.SourceProgressExportJobReady:
					t.Fatal("invalid job reported ready")
				}
			}
			if created != tc.created || polled != tc.polled {
				t.Fatalf("unexpected reports: %v", reports)
			}
		})
	}
}

func TestDirectDownloadReportsAvailabilityWithoutCreation(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `[]`) }))
	t.Cleanup(api.Close)
	var reports []filament.SourceProgressKind
	c := &Runtime{Transport: api.Client().Transport, Observe: func(p filament.SourceProgress) { reports = append(reports, p.Kind) }}
	res := recoveryResource()
	res.Parent = nil
	res.Export.Direct = true
	res.Export.Wait.Type = "download"
	res.Export.Result.URL = api.URL
	if err := c.Run(t.Context(), res, &snapshotSink{}, nil); err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0] != filament.SourceProgressExportJobPolled || reports[1] != filament.SourceProgressExportJobReady {
		t.Fatalf("direct download events = %v", reports)
	}
}

func TestScopedResumeStillHonorsCancellation(t *testing.T) {
	control := &recoveryControl{}
	c, res := &Runtime{Control: control}, recoveryResource()
	state := recoveryState(c, res, false, time.Now().Add(-time.Hour), "waiting")
	var err error
	c.ResumeToken, err = state.token()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := c.Run(ctx, res, &snapshotSink{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled recovery = %v", err)
	}
	if control.polls != 0 || control.starts != 0 {
		t.Fatal("canceled recovery contacted upstream")
	}
}
