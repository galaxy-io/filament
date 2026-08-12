// Package incremental implements watermark-based incremental extraction.
//
// A Tracker observes records as they flow through the pipeline, tracks the
// maximum value of a declared cursor field (e.g. `updated_at`), and injects
// that value into the next run's initial request via query, body, or header.
// The last observed value persists to the PipelineCheckpoint between runs.
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
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline/integrity"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

// Tracker runs alongside a resource extraction. Safe for concurrent Observe.
type Tracker struct {
	spec     manifest.IncrementalSpec
	resource string

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

// CursorField returns the manifest-declared cursor field name. Used by
// observability surfaces (Reporter events, structured logs) so dashboards
// don't have to plumb the spec separately.
func (t *Tracker) CursorField() string { return t.spec.CursorField }

// CheckpointKey returns the durable storage key for this tracker.
func (t *Tracker) CheckpointKey() string { return t.checkpointKey() }

// CheckpointKey resolves the durable storage key for an incremental spec.
func CheckpointKey(spec manifest.IncrementalSpec) string { return checkpointKey(spec) }

// Scope returns a {start_param: effective_value} pair suitable for merging
// into a template scope's State map. The effective value is the current
// watermark minus OverlapSeconds (time or numeric timestamp comparator). Returns nil when no
// watermark is set yet.
func (t *Tracker) Scope() map[string]string {
	v := t.effective()
	if v == "" {
		return nil
	}
	return map[string]string{t.spec.StartParam: v}
}

// effective returns the watermark value to inject — current minus overlap
// for the time comparator, current as-is otherwise.
func (t *Tracker) effective() string {
	v := t.Current()
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

// Apply injects the current watermark into an outgoing request. For
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

// Commit persists the current watermark into the checkpoint. Goes through
// MergeWatermark so concurrent commits from fan-out parents converge on the
// max watermark per the configured comparator.
//
// Returns an error when the comparator can't be resolved, when the current
// watermark itself fails to parse under that comparator, or when an existing
// checkpoint value can't be parsed (a stale checkpoint written under a
// different comparator). Callers must surface the error — silently preferring
// "the new value" mid-extraction would advance past whatever real watermark
// the checkpoint held, breaking at-least-once semantics.
func (t *Tracker) Commit(pc *integrity.PipelineCheckpoint) error {
	if pc == nil {
		return nil
	}
	current := t.Current()
	if current == "" {
		return nil
	}
	cmp, err := atomicwatermark.ForName(t.spec.Comparator)
	if err != nil {
		return fmt.Errorf("incremental: commit %s: %w", t.resource, err)
	}
	// Validate current parses under the comparator before storing — a Tracker
	// constructed with a bad initial wouldn't have surfaced here otherwise.
	if _, err := cmp.Less(current, current); err != nil {
		return fmt.Errorf("incremental: commit %s: current watermark %q invalid under %s: %w",
			t.resource, current, cmp.Name(), err)
	}
	var cmpErr error
	pc.MergeWatermark(t.resource, t.checkpointKey(), current, func(a, b string) int {
		// MergeWatermark wants -1/0/1; map "a < b → -1" to atomicwatermark.Less.
		less, err := cmp.Less(a, b)
		if err != nil {
			// Capture and abort the merge: the closure's return value is
			// ignored once cmpErr is set since MergeWatermark has no error
			// channel. Returning 0 keeps the existing value, which is the
			// safer default — a corrupt new value must not overwrite a
			// previously-good one.
			cmpErr = fmt.Errorf("incremental: commit %s: comparator %s rejected value: %w",
				t.resource, cmp.Name(), err)
			return 0
		}
		if less {
			return -1
		}
		eq, _ := cmp.Less(b, a)
		if !eq {
			return 0
		}
		return 1
	})
	return cmpErr
}

// LoadFrom reads the committed watermark from the checkpoint. Returns "" if
// none is stored.
func LoadFrom(pc *integrity.PipelineCheckpoint, resource string, spec manifest.IncrementalSpec) string {
	if pc == nil {
		return ""
	}
	return pc.GetWatermark(resource, checkpointKey(spec))
}

// checkpointKey resolves the checkpoint storage key for a spec. Callers that
// have a Tracker should use Tracker.checkpointKey() to avoid plumbing the
// spec through.
func checkpointKey(spec manifest.IncrementalSpec) string {
	if spec.CheckpointKey != "" {
		return spec.CheckpointKey
	}
	return spec.CursorField
}

func (t *Tracker) checkpointKey() string { return checkpointKey(t.spec) }
