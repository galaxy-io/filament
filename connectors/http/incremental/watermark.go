// Package incremental implements watermark-based incremental extraction.
//
// A Tracker observes records as they flow through the pipeline, tracks the
// maximum value of a declared cursor field (e.g. `updated_at`), and injects
// that value into the next run's initial request via query, body, or header.
// The last observed value is attached to emitted records so the engine can
// persist it in the resource checkpoint between runs.
//
// # Comparator
//
// Comparator selects ordering and is validated at construction:
//
//   - "lex" (default) — string comparison; correct for RFC3339 + zero-padded ids
//   - "numeric"       — float64 parse on both sides; errors on parse failure
//   - "time"          — RFC3339 parse on both sides; errors on parse failure
//
// Extraction uses ObserveChecked, so a missing or unparseable durable cursor
// fails the run rather than letting it succeed without checkpoint progress.
// Observe remains available as a best-effort compatibility helper.
//
// # Concurrency
//
// Observe is safe to call from any number of goroutines (fan-out across
// child resources). Internally Tracker uses a CAS loop in atomicwatermark, so
// the stored value is always the max regardless of arrival order.
//
// # Overlap window (time or numeric timestamp comparator)
//
// OverlapSeconds re-fetches a sliding window before the persisted cursor on
// Apply. Catches retroactive updates whose timestamps fall behind the last
// observed max. Worked example: cursor T at 12:00, OverlapSeconds=3600 →
// the next request asks for `updated_at >= 11:00`, so any record updated in
// [11:00, 12:00) that wasn't visible during the last run still surfaces.
// Choose OverlapSeconds ≥ max-expected clock skew between source and reader.
package incremental

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/galaxy-io/filament/connectors/http/internal/atomicwatermark"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

// Tracker runs alongside a resource extraction. Safe for concurrent Observe.
type Tracker struct {
	spec     manifest.IncrementalSpec
	resource string
	// start is the immutable lower bound for this extraction. The running
	// watermark advances as records arrive, but changing the request filter
	// between pages can invalidate a server cursor and skip records.
	start string

	wm     *atomicwatermark.Watermark
	logger *slog.Logger
}

// Option customizes a Tracker at construction.
type Option func(*Tracker)

// WithLogger attaches a logger that records bad cursor values (failed parse)
// and other non-fatal anomalies. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(t *Tracker) { t.logger = obs.Logger(l) }
}

// New builds a Tracker. initialWatermark seeds from a stored checkpoint;
// falls back to spec.Initial when empty.
func New(spec manifest.IncrementalSpec, resource, initialWatermark string, opts ...Option) (*Tracker, error) {
	if spec.CursorField == "" {
		return nil, fmt.Errorf("incremental: cursor_field is required")
	}
	if spec.StartParam == "" {
		return nil, fmt.Errorf("incremental: start_param is required")
	}
	switch spec.InjectInto {
	case "query", "body", "header":
	default:
		return nil, fmt.Errorf("incremental: inject_into must be query|body|header (got %q)", spec.InjectInto)
	}
	cmp, err := atomicwatermark.ForName(spec.Comparator)
	if err != nil {
		return nil, fmt.Errorf("incremental: %w", err)
	}
	if spec.OverlapSeconds < 0 {
		return nil, fmt.Errorf("incremental: overlap_seconds must be non-negative")
	}
	if spec.OverlapSeconds > 0 && spec.Comparator != "time" && spec.Comparator != "numeric" {
		return nil, fmt.Errorf("incremental: overlap_seconds requires comparator: time or numeric")
	}
	start := initialWatermark
	if start == "" {
		start = spec.Initial
	}
	wm, err := atomicwatermark.New(cmp, start)
	if err != nil {
		return nil, fmt.Errorf("incremental: %w", err)
	}
	t := &Tracker{
		spec:     spec,
		resource: resource,
		start:    start,
		wm:       wm,
	}
	for _, opt := range opts {
		opt(t)
	}
	// Apply default after opts so an explicit WithLogger(nil) still lands on
	// slog.Default() rather than nil-panicking on the first Warn call.
	t.logger = obs.Logger(t.logger)
	return t, nil
}

// Observe inspects one record's cursor field and advances the watermark. It is
// the compatibility, best-effort form; extraction uses ObserveChecked so bad
// durable cursor data fails the run instead of silently stalling progress.
func (t *Tracker) Observe(record map[string]any) bool {
	advanced, err := t.ObserveChecked(record)
	if err != nil {
		t.logger.Warn("incremental: cursor value rejected",
			"resource", t.resource, "field", t.spec.CursorField, "error", err)
		return false
	}
	return advanced
}

// ObserveChecked advances the watermark or returns a cursor lookup/type error.
// A durable incremental run must not report success when its configured cursor
// is missing or unorderable on a returned record.
func (t *Tracker) ObserveChecked(record map[string]any) (bool, error) {
	cursorPath := t.spec.CursorPath
	if cursorPath == "" {
		cursorPath = t.spec.CursorField
	}
	v, _, err := paths.AsString(record, cursorPath)
	if err != nil {
		return false, fmt.Errorf("cursor field %q: %w", t.spec.CursorField, err)
	}
	if v == "" {
		return false, fmt.Errorf("cursor field %q is empty", t.spec.CursorField)
	}
	advanced, err := t.wm.Observe(v)
	if err != nil {
		return false, fmt.Errorf("cursor field %q value %q rejected by %s comparator: %w", t.spec.CursorField, v, t.wm.ComparatorName(), err)
	}
	return advanced, nil
}

// Current returns the running watermark (max observed or initial).
func (t *Tracker) Current() string { return t.wm.Current() }

// Scope returns a {start_param: effective_start} pair suitable for merging
// into a template scope's State map. The lower bound is fixed for the entire
// extraction so paginated requests all scan the same result set.
func (t *Tracker) Scope() map[string]string {
	v := t.effective()
	if v == "" {
		return nil
	}
	return map[string]string{t.spec.StartParam: v}
}

// effective returns the extraction's fixed starting watermark minus overlap
// for time/numeric comparators.
func (t *Tracker) effective() string {
	v := t.start
	if v == "" || t.spec.OverlapSeconds == 0 {
		return v
	}
	if t.spec.Comparator == "numeric" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.logger.Warn("incremental: cannot apply overlap to non-numeric watermark",
				"resource", t.resource, "value", v, "error", err)
			return v
		}
		return strconv.FormatFloat(parsed-float64(t.spec.OverlapSeconds), 'f', -1, 64)
	}
	if t.spec.Comparator != "time" {
		return v
	}
	parsed, err := time.Parse(time.RFC3339, v)
	if err != nil {
		// Only reached if the checkpoint stores a non-RFC3339 watermark
		// under a time comparator; Observe rejects such values going
		// forward, but a malformed checkpoint is logged rather than
		// erroring so extraction can still progress.
		t.logger.Warn("incremental: cannot apply overlap to non-RFC3339 watermark",
			"resource", t.resource, "value", v, "error", err)
		return v
	}
	return parsed.Add(-time.Duration(t.spec.OverlapSeconds) * time.Second).Format(time.RFC3339)
}

// Apply injects the extraction's fixed starting watermark into a request. For
// body-injection, returns a body overrides map to be merged before encoding.
// Returns nil overrides for query/header strategies.
func (t *Tracker) Apply(req *http.Request) (map[string]any, error) {
	v := t.effective()
	if v == "" {
		return nil, nil
	}
	switch t.spec.InjectInto {
	case "query":
		q := req.URL.Query()
		q.Set(t.spec.StartParam, v)
		req.URL.RawQuery = q.Encode()
	case "header":
		req.Header.Set(t.spec.StartParam, v)
	case "body":
		return map[string]any{t.spec.StartParam: v}, nil
	}
	return nil, nil
}
