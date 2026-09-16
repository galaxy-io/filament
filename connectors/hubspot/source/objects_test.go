package hubspot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/pipeline"
)

var testTime = time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

// The fixture reverses batch results and changes their timestamps after search.
// This catches both positional hydration and accidentally checkpointing hydrated data.
type objectFixture struct {
	t              *testing.T
	count          int
	modified       string
	selected       time.Time
	searches       []searchRequest
	lists          int
	missing        bool
	failAfterFirst bool
}

func (f *objectFixture) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Authorization") != "Bearer test" {
		f.t.Error("missing authorization")
	}
	w.Header().Set("Content-Type", "application/json")
	var out any
	switch {
	case strings.Contains(req.URL.Path, "/properties/"):
		out = map[string]any{"results": []any{
			map[string]any{"name": f.modified},
			map[string]any{"name": "custom_text"},
			map[string]any{"name": "secret_property", "dataSensitivity": "sensitive"},
		}}
	case strings.HasSuffix(req.URL.Path, "/batch/read"):
		var body struct {
			Properties []string
			Inputs     []struct{ ID string }
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			f.t.Error(err)
		}
		if !slices.Contains(body.Properties, "custom_text") || slices.Contains(body.Properties, "secret_property") {
			f.t.Errorf("properties = %v", body.Properties)
		}
		rows := make([]object, 0, len(body.Inputs))
		for _, input := range body.Inputs {
			rows = append(rows, f.row(input.ID, testTime.Add(time.Hour)))
		}
		slices.Reverse(rows)
		if f.missing && len(rows) > 0 {
			rows = rows[1:]
		}
		out = map[string]any{"status": "COMPLETE", "results": rows}
	case strings.HasSuffix(req.URL.Path, "/search"):
		if req.Method != http.MethodPost {
			f.t.Error("search must use POST")
		}
		var body searchRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			f.t.Error(err)
		}
		f.searches = append(f.searches, body)
		if f.failAfterFirst && len(f.searches) > 1 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"category":"VALIDATION_ERROR"}`))
			return
		}
		after := 0
		for _, filter := range body.FilterGroups[0].Filters {
			if filter.Property == "hs_object_id" {
				after, _ = strconv.Atoi(filter.Value)
			}
		}
		rows := make([]object, 0)
		selected := f.selected
		if selected.IsZero() {
			selected = testTime.Add(-time.Minute)
		}
		for id := after + 1; id <= min(after+body.Limit, f.count); id++ {
			rows = append(rows, f.row(strconv.Itoa(id), selected))
		}
		out = map[string]any{"results": rows, "total": f.count}
	default:
		f.lists++
		after, _ := strconv.Atoi(req.URL.Query().Get("after"))
		rows := make([]object, 0)
		for id := after + 1; id <= min(after+100, f.count); id++ {
			rows = append(rows, f.row(strconv.Itoa(id), testTime.Add(-time.Minute)))
		}
		page := objectPage{Results: rows}
		if after+100 < f.count {
			page.Paging.Next.After = strconv.Itoa(after + 100)
		}
		out = page
	}
	if err := json.NewEncoder(w).Encode(out); err != nil {
		f.t.Error(err)
	}
}

func (f *objectFixture) row(id string, modified time.Time) object {
	properties, _ := json.Marshal(map[string]string{f.modified: modified.Format(time.RFC3339Nano), "custom_text": "value-" + id})
	return object{ID: id, CreatedAt: testTime.Add(-time.Hour), UpdatedAt: modified, Properties: properties}
}

func fixtureSource(t *testing.T, f *objectFixture) *Source {
	t.Helper()
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	s := New()
	if err := s.Configure(t.Context(), filament.NewConfig(map[string]any{"api_key": "test"})); err != nil {
		t.Fatal(err)
	}
	s.client.baseURL = server.URL
	s.client.limiter = rate.NewLimiter(rate.Inf, 1)
	s.now = func() time.Time { return testTime }
	t.Cleanup(func() { _ = s.Teardown(context.Background()) })
	return s
}

type capturedRow struct {
	id, properties string
	key            []string
}
type captureSink struct {
	mu                      sync.Mutex
	rows                    []capturedRow
	applyError, commitError error
}

func (*captureSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "capture", Capabilities: filament.SinkCapabilities{
		WritePolicies: filament.WriteCapabilities(filament.IngestionIncrementalUpsert, filament.IngestionFullUpsert),
	}}
}
func (*captureSink) Name() string                                 { return "capture" }
func (*captureSink) Open(context.Context, filament.RunSpec) error { return nil }
func (s *captureSink) Commit(context.Context) error               { return s.commitError }
func (*captureSink) Abort(context.Context) error                  { return nil }
func (s *captureSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.applyError != nil {
		return filament.WriteReceipt{}, s.applyError
	}
	for i := 0; i < b.NumRows(); i++ {
		s.rows = append(s.rows, capturedRow{
			id:         fmt.Sprint(b.Rows().Column(0).GetOneForMarshal(i)),
			properties: fmt.Sprint(b.Rows().Column(5).GetOneForMarshal(i)),
			key:        slices.Clone(b.Last.Key),
		})
	}
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func extractFixture(t *testing.T, s *Source, name string, limit int, previous time.Time) (*captureSink, error) {
	t.Helper()
	plans, err := s.PlanIncremental(t.Context(), []string{name}, map[string]filament.Checkpoint{name: watermarkPlan(name, previous)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sink := &captureSink{}
	plan, err := filament.ResolveIngestionPlan(t.Context(), s, sink, filament.RunSpec{Resources: []string{name}, IngestionTypes: map[string]filament.IngestionType{name: filament.IngestionIncrementalUpsert}})
	if err != nil {
		t.Fatal(err)
	}
	p := pipeline.New(pipeline.Config{Sink: sink, WritePolicies: plan.WritePolicies, Options: filament.RunOptions{BatchMaxRows: 1}})
	p.Start(t.Context())
	err = s.ExtractFrom(t.Context(), p.Records(), filament.ExtractOpts{Resources: []string{name}, Limit: limit}, plans)
	p.CloseIngest(err)
	if waitErr := p.Wait(); waitErr != nil {
		t.Fatal(waitErr)
	}
	return sink, err
}

func TestIncrementalIDTraversalBeyondSearchLimit(t *testing.T) {
	f := &objectFixture{t: t, count: 10201, modified: "lastmodifieddate"}
	s := fixtureSource(t, f)
	sink, err := extractFixture(t, s, "contacts", 0, testTime.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.rows) != f.count || f.lists != 0 || len(f.searches) != 53 {
		t.Fatalf("rows=%d lists=%d searches=%d", len(sink.rows), f.lists, len(f.searches))
	}
	for i, row := range sink.rows {
		if row.id != strconv.Itoa(i+1) || !strings.Contains(row.properties, "value-"+row.id) {
			t.Fatalf("misaligned row %d: %+v", i, row)
		}
	}
	want := testTime.Add(-time.Minute).Format(time.RFC3339Nano)
	if got := sink.rows[len(sink.rows)-1].key; !slices.Equal(got, []string{want}) {
		t.Fatalf("checkpoint = %v, want %s", got, want)
	}
	first := f.searches[0]
	if !slices.Equal(first.Sorts, []string{"hs_object_id"}) {
		t.Fatal(first.Sorts)
	}
	filters := first.FilterGroups[0].Filters
	if filters[0].Value != testTime.Add(-time.Hour-defaultLookback).Format(time.RFC3339Nano) || filters[1].Value != testTime.Format(time.RFC3339Nano) {
		t.Fatal(filters)
	}
}

func TestWatermarkRequiresExhaustion(t *testing.T) {
	for _, test := range []struct {
		name                     string
		count, limit             int
		fail, missing, bootstrap bool
		wantRows                 int
		wantError                bool
	}{
		{name: "row limit", count: 250, limit: 201, wantRows: 201},
		{name: "exact limit", count: 200, limit: 200, wantRows: 200},
		{name: "partial search", count: 250, fail: true, wantRows: 199, wantError: true},
		{name: "missing hydration", count: 2, missing: true, wantRows: 0, wantError: true},
		{name: "empty interval", count: 0, wantRows: 0},
		{name: "limited bootstrap", count: 250, limit: 120, bootstrap: true, wantRows: 120},
		{name: "complete bootstrap", count: 250, bootstrap: true, wantRows: 250},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &objectFixture{t: t, count: test.count, modified: "hs_lastmodifieddate", failAfterFirst: test.fail, missing: test.missing}
			s := fixtureSource(t, f)
			previous := testTime.Add(-time.Hour)
			if test.bootstrap {
				previous = initialWatermark
			}
			sink, err := extractFixture(t, s, "custom_objects_2_123", test.limit, previous)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v", err)
			}
			if len(sink.rows) != test.wantRows {
				t.Fatalf("rows = %d, want %d", len(sink.rows), test.wantRows)
			}
			for i, row := range sink.rows {
				var want []string
				if test.bootstrap && test.limit == 0 && i == len(sink.rows)-1 {
					want = []string{testTime.Format(time.RFC3339Nano)}
				}
				if !slices.Equal(row.key, want) {
					t.Fatalf("row %d checkpoint = %v, want %v", i, row.key, want)
				}
			}
		})
	}
}

func TestSearchRejectsUnsafeOrderingAndTimestamps(t *testing.T) {
	f := &objectFixture{modified: "lastmodifieddate"}
	for _, rows := range [][]object{
		{f.row("2", testTime), f.row("1", testTime)},
		{f.row("1", testTime), f.row("1", testTime)},
		{f.row("1", testTime.Add(time.Hour))},
		{{ID: "1", Properties: json.RawMessage(`{}`)}},
	} {
		if err := validateSearchPage(rows, "", f.modified, testTime.Add(-time.Hour), testTime.Add(time.Minute)); err == nil {
			t.Fatal("accepted unsafe search page")
		}
	}
}
