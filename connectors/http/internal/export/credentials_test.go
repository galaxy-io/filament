package export

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

type snapshotSink struct {
	rejectingSink
	tokens []string
	err    error
	rows   int
}

func (s *snapshotSink) Checkpoint(_, token string) error {
	if s.err != nil {
		return s.err
	}
	s.tokens = append(s.tokens, token)
	return nil
}

func (s *snapshotSink) Emit(map[string]any, map[string]string, string) (int, error) {
	s.rows++
	return 1, nil
}

func recoveryResource() manifest.Resource {
	return manifest.Resource{Name: "items", Parent: &manifest.ParentRef{Concurrency: 1}, Export: &manifest.ExportSpec{
		ParentKey: []string{"id"},
		Start:     manifest.ExportStart{ExportRequest: manifest.ExportRequest{Method: "POST", Path: "/exports"}, Capture: map[string]string{"id": "$.id"}},
		Wait: manifest.ExportWait{
			Type: "job", TimeoutSeconds: 30, Request: &manifest.ExportRequest{Method: "GET", Path: "/status"},
			State: &manifest.ExportJobState{Path: "$.status", Ready: []string{"ready"}, Pending: []string{"pending"}}, Capture: map[string]string{"url": "$.url"},
		},
		Result: manifest.ExportResult{URL: "{{ job.url }}", AllowedHosts: []string{"127.0.0.1"}, Format: "json"},
	}}
}

func recoveryState(c *Runtime, res manifest.Resource, inc bool, deadline time.Time, phases ...string) RunState {
	s := c.emptyExportRun(res, inc)
	s.Initialized = true
	for i, phase := range phases {
		key := fmt.Sprint(i)
		s.Selected = append(s.Selected, key)
		s.Parents[key] = exportParentState{Parent: map[string]string{"id": key}, Committed: "committed", Job: &exportScopedJob{
			Phase: phase, Deadline: deadline, Captures: map[string]string{"id": "sensitive-job-" + key}, Candidate: "candidate",
		}}
	}
	return s
}

type recoveryControl struct {
	polls, starts               int
	url                         string
	startResponse, pollResponse string
}

func (c *recoveryControl) Start(context.Context, manifest.Resource, template.Scope) ([]byte, error) {
	c.starts++
	if c.startResponse != "" {
		return []byte(c.startResponse), nil
	}
	return []byte(`{"id":"sensitive-created-job"}`), nil
}

func (c *recoveryControl) Poll(ctx context.Context, _ string, _ manifest.ExportRequest, _ template.Scope) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.polls++
	if c.pollResponse != "" {
		return []byte(c.pollResponse), nil
	}
	return []byte(fmt.Sprintf(`{"status":"ready","url":%q}`, c.url)), nil
}

func credentialResource() manifest.Resource {
	res := recoveryResource()
	res.Export.Start.Path = "/exports/{{ config.api_key }}"
	res.Export.Start.Query = map[string]string{"key": "{{ config.api_key }}", "region": "{{ config.region }}"}
	res.Export.Start.Headers = map[string]string{"X-Key": "{{ env.EXPORT_KEY }}"}
	res.Export.Start.Body = manifest.BodySpec{Encoding: "json", Template: map[string]any{"key": "{{ config.api_key }}", "parent": "{{ parent.id }}"}}
	return res
}

func credentialRuntime() *Runtime {
	return &Runtime{Config: map[string]string{"api_key": "old-private-key", "region": "west"}, Env: map[string]string{"EXPORT_KEY": "old-env-key"}, IdentityConfig: map[string]string{"region": "west"}}
}

type credentialControl struct {
	recoveryControl
	seen manifest.Resource
}

func (c *credentialControl) Start(ctx context.Context, res manifest.Resource, scope template.Scope) ([]byte, error) {
	c.seen = res
	return c.recoveryControl.Start(ctx, res, scope)
}

func TestPlannedExportResolvesRotatedCredentialsWithoutPersistingThem(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `[]`) }))
	t.Cleanup(api.Close)
	c, res := credentialRuntime(), credentialResource()
	state := c.emptyExportRun(res, false)
	if err := c.initializeExportParents(res, &state, []map[string]string{{"id": "original-parent"}}); err != nil {
		t.Fatal(err)
	}
	var err error
	c.ResumeToken, err = state.token()
	if err != nil {
		t.Fatal(err)
	}
	c.Config["api_key"], c.Env["EXPORT_KEY"] = "rotated-private-key", "rotated-env-key"
	control := &credentialControl{recoveryControl: recoveryControl{url: api.URL}}
	c.Control, c.Transport = control, api.Client().Transport
	sink := &snapshotSink{}
	if err := c.Run(t.Context(), res, sink, []map[string]string{{"id": "changed-parent"}}); err != nil {
		t.Fatal(err)
	}
	if control.starts != 1 || control.seen.Path != "/exports/rotated-private-key" || control.seen.Query["key"] != "rotated-private-key" || control.seen.Query["region"] != "west" || control.seen.Headers["X-Key"] != "rotated-env-key" {
		t.Fatal("creation did not resolve current credentials")
	}
	body := control.seen.Body.Template.(map[string]any)
	if body["key"] != "rotated-private-key" || body["parent"] != "original-parent" {
		t.Fatal("body credentials or frozen parent changed")
	}
	for _, token := range append(sink.tokens, c.ResumeToken) {
		if strings.Contains(token, `"start":`) || strings.Contains(token, `"start_deferred":`) {
			t.Fatal("checkpoint retained a request or credential bindings")
		}
		for _, secret := range []string{"old-private-key", "old-env-key", "rotated-private-key", "rotated-env-key"} {
			if strings.Contains(token, secret) {
				t.Fatal("credential entered a checkpoint")
			}
		}
		if _, err := c.decodeExportRun(res, token, false); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMissingCreationCredentialDoesNotPublishCreationIntent(t *testing.T) {
	c, res := credentialRuntime(), credentialResource()
	delete(c.Config, "api_key")
	control := &recoveryControl{}
	c.Control = control
	sink := &snapshotSink{}
	if err := c.Run(t.Context(), res, sink, []map[string]string{{"id": "parent"}}); err == nil {
		t.Fatal("missing credential accepted")
	}
	if control.starts != 0 || control.polls != 0 {
		t.Fatal("missing credential contacted upstream")
	}
	if len(sink.tokens) != 1 {
		t.Fatalf("checkpoints = %d, want initial plan only", len(sink.tokens))
	}
	state, err := c.decodeExportRun(res, sink.tokens[0], false)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range state.Parents {
		if entry.Job.Phase != "planned" {
			t.Fatal("local rendering error created an ambiguous intent")
		}
	}
}

func TestRebuiltExportStartPreservesSeedAndLiteralInputs(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `[]`) }))
	t.Cleanup(api.Close)
	for _, injection := range []string{"query", "header", "body"} {
		t.Run(injection, func(t *testing.T) {
			c, res := credentialRuntime(), credentialResource()
			res.Fields = manifest.FieldList{{Name: "sequence", Path: "sequence", Type: "int64"}}
			res.Incremental = &manifest.IncrementalSpec{CursorField: "sequence", StartParam: "since", InjectInto: injection, Initial: "10", Comparator: "numeric", OverlapSeconds: 2}
			state := c.emptyExportRun(res, true)
			if err := c.initializeExportParents(res, &state, []map[string]string{{"id": "{{ config.api_key }}"}}); err != nil {
				t.Fatal(err)
			}
			var err error
			c.ResumeToken, err = state.token()
			if err != nil {
				t.Fatal(err)
			}
			// A new run's lookback option must not replace the saved job's overlap.
			lookback := 7
			c.Incremental, c.Lookback = true, &lookback
			c.Config["api_key"] = "rotated-{{ parent.missing }}"
			control := &credentialControl{recoveryControl: recoveryControl{url: api.URL}}
			c.Control, c.Transport = control, api.Client().Transport
			if err := c.Run(t.Context(), res, &snapshotSink{}, nil); err != nil {
				t.Fatal(err)
			}
			body := control.seen.Body.Template.(map[string]any)
			if body["parent"] != "{{ config.api_key }}" || body["key"] != c.Config["api_key"] {
				t.Fatal("request input was evaluated more than once")
			}
			got := ""
			switch injection {
			case "query":
				got = control.seen.Query["since"]
			case "header":
				got = control.seen.Headers["Since"]
			case "body":
				got, _ = body["since"].(string)
			}
			if got != "8" {
				t.Fatalf("rebuilt lower bound = %q, want frozen 10 - 2", got)
			}
		})
	}
}

func TestCapturedExportDoesNotRenderCreationRequest(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `[]`) }))
	t.Cleanup(api.Close)
	for _, phase := range []string{"waiting", "done", "creating"} {
		t.Run(phase, func(t *testing.T) {
			c, res := credentialRuntime(), credentialResource()
			state := recoveryState(c, res, false, time.Now().Add(time.Minute), phase)
			var err error
			c.ResumeToken, err = state.token()
			if err != nil {
				t.Fatal(err)
			}
			delete(c.Config, "api_key")
			control := &credentialControl{recoveryControl: recoveryControl{url: api.URL}}
			c.Control, c.Transport = control, api.Client().Transport
			err = c.Run(t.Context(), res, &snapshotSink{}, nil)
			if phase == "creating" {
				if err == nil || !strings.Contains(err.Error(), "reconcile") || control.polls != 0 {
					t.Fatalf("ambiguous creation = %v", err)
				}
			} else if err != nil || control.polls != 1 {
				t.Fatalf("captured job replay = %v", err)
			}
			if control.starts != 0 {
				t.Fatal("captured job was recreated")
			}
		})
	}
}

func TestInputOnlyCheckpointRetainsIdentityAndStrictDecoding(t *testing.T) {
	c, res := credentialRuntime(), credentialResource()
	state := recoveryState(c, res, false, time.Now(), "planned")
	token, err := state.token()
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.Replace(token, `"phase":"planned"`, `"unexpected":true,"phase":"planned"`, 1)
	if _, err := c.decodeExportRun(res, unknown, false); err == nil {
		t.Fatal("unknown job field was accepted")
	}
	c.IdentityConfig["region"] = "east"
	if _, err := c.decodeExportRun(res, token, false); err == nil {
		t.Fatal("changed non-secret configuration was accepted")
	}
	c.IdentityConfig["region"] = "west"
	res.Export.Start.Path = "/changed"
	if _, err := c.decodeExportRun(res, token, false); err == nil {
		t.Fatal("changed manifest was accepted")
	}
}
