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
//   - "time"          — timestamp parse on both sides; errors on parse failure.
//     Cursor values are normalized to RFC3339 before the tracker sees them,
//     so APIs that emit colon-less offsets (Jira) are accepted and the stored
//     checkpoint, the overlap window, and the wire format all see one form.
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
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/galaxy-io/filament/connectors/http/internal/atomicwatermark"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/internal/timestamps"
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
	// end is the extraction's upper bound when the spec declares end_param:
	// the RFC3339 time the tracker was created less end_lag_seconds, fixed
	// for the whole run.
	end string
	now func() time.Time
	// windowOnly trackers send the request window on a full refresh but
	// never observe records, so they report no watermark and filter nothing.
	windowOnly bool

	wm     *atomicwatermark.Watermark
	cmp    atomicwatermark.Comparator
	logger *slog.Logger
}

// Option customizes a Tracker at construction.
type Option func(*Tracker)

// WithLogger attaches a logger that records bad cursor values (failed parse)
// and other non-fatal anomalies. Defaults to slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(t *Tracker) { t.logger = obs.Logger(l) }
}

// WithClock replaces the clock that fixes the end_param upper bound. Tests use
// it to pin the window end.
func WithClock(now func() time.Time) Option {
	return func(t *Tracker) { t.now = now }
}

// WindowOnly makes the tracker send only its request window: start_param
// from the initial value and end_param. Records are not observed, so the run
// reports no watermark. Full refreshes of resources with end_param use it,
// since APIs that need an end date need it on every read.
func WindowOnly() Option {
	return func(t *Tracker) { t.windowOnly = true }
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
	case "query", "body", "header", "state":
	default:
		return nil, fmt.Errorf("incremental: inject_into must be query|body|header|state (got %q)", spec.InjectInto)
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
	if spec.ValueDateFormat != "" && spec.Comparator != "time" {
		return nil, fmt.Errorf("incremental: value_date_format requires comparator: time")
	}
	if err := manifest.ValidateValueDateFormat(spec.ValueDateFormat); err != nil {
		return nil, fmt.Errorf("incremental: %w", err)
	}
	if spec.EndParam != "" && spec.Comparator != "time" {
		return nil, fmt.Errorf("incremental: end_param requires comparator: time")
	}
	if spec.EndLagSeconds < 0 || (spec.EndLagSeconds > 0 && spec.EndParam == "") {
		return nil, fmt.Errorf("incremental: end_lag_seconds must be 0 or greater and requires end_param")
	}
	start := initialWatermark
	if start == "" {
		start = spec.Initial
	}
	// Normalize a time-comparator start to RFC3339 before anything else
	// sees it, so a checkpoint written before this fix (or an initial in a
	// colon-less spelling) does not fail the overlap and wire-format paths
	// that parse strict RFC3339. A bad start fails at setup.
	if spec.Comparator == "time" && start != "" {
		normalized, err := timestamps.Normalize(start)
		if err != nil {
			return nil, fmt.Errorf("incremental: initial watermark %q is not a timestamp: %w", start, err)
		}
		start = normalized
	}
	// Fail at setup when the initial watermark cannot be formatted with
	// the configured wire format, instead of logging a warning per
	// request and sending the raw RFC3339 value to an API that rejects it.
	if spec.ValueDateFormat != "" && start != "" {
		if _, err := formatWatermark(start, spec.ValueDateFormat); err != nil {
			return nil, fmt.Errorf("incremental: initial watermark %q cannot be formatted with value_date_format %q: %w", start, spec.ValueDateFormat, err)
		}
	}
	wm, err := atomicwatermark.New(cmp, start)
	if err != nil {
		return nil, fmt.Errorf("incremental: %w", err)
	}
	t := &Tracker{
		spec:     spec,
		resource: resource,
		start:    start,
		now:      time.Now,
		wm:       wm,
		cmp:      cmp,
	}
	for _, opt := range opts {
		opt(t)
	}
	if spec.EndParam != "" {
		lag := time.Duration(spec.EndLagSeconds) * time.Second
		t.end = t.now().UTC().Add(-lag).Truncate(time.Second).Format(time.RFC3339)
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
	if t.windowOnly {
		return false, nil
	}
	v, err := t.Cursor(record)
	if err != nil {
		return false, err
	}
	advanced, err := t.wm.Observe(v)
	if err != nil {
		return false, fmt.Errorf("cursor field %q value %q rejected by %s comparator: %w", t.spec.CursorField, v, t.wm.ComparatorName(), err)
	}
	return advanced, nil
}

// Cursor reads a record's cursor value in the form the tracker compares and
// stores. A time cursor is normalized to RFC3339 so the stored checkpoint,
// the overlap window, and the wire format all see one form, and a JSON
// number is read as a Unix epoch with the rule timestamptz columns use
// (seconds, or milliseconds above 1e12). Strings, including digit strings,
// parse only as timestamps.
func (t *Tracker) Cursor(record map[string]any) (string, error) {
	cursorPath := t.spec.CursorPath
	if cursorPath == "" {
		cursorPath = t.spec.CursorField
	}
	if t.spec.Comparator == "time" {
		if raw, err := paths.Value(record, cursorPath); err == nil {
			if f, ok := epochNumber(raw); ok {
				return timestamps.NormalizeEpoch(f), nil
			}
		}
	}
	v, _, err := paths.AsString(record, cursorPath)
	if err != nil {
		return "", fmt.Errorf("cursor field %q: %w", t.spec.CursorField, err)
	}
	if v == "" {
		return "", fmt.Errorf("cursor field %q is empty", t.spec.CursorField)
	}
	// Without normalization, an API spelling like Jira's colon-less offset
	// fails the comparator and the first incremental run dies on its first
	// record.
	if t.spec.Comparator == "time" {
		normalized, err := timestamps.Normalize(v)
		if err != nil {
			return "", fmt.Errorf("cursor field %q value %q is not a timestamp: %w", t.spec.CursorField, v, err)
		}
		v = normalized
	}
	return v, nil
}

// epochNumber reports the value of a decoded JSON number.
func epochNumber(raw any) (float64, bool) {
	switch n := raw.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// Current returns the running watermark (max observed or initial). A
// window-only tracker has none.
func (t *Tracker) Current() string {
	if t.windowOnly {
		return ""
	}
	return t.wm.Current()
}

// Start returns the extraction's effective lower bound: the seeded watermark
// minus overlap. Empty when the run has no starting point.
func (t *Tracker) Start() string { return t.effective() }

// Below reports whether v sorts before Start under the tracker's comparator.
// An empty Start admits every value.
func (t *Tracker) Below(v string) (bool, error) {
	start := t.effective()
	if start == "" || t.windowOnly {
		return false, nil
	}
	if t.spec.Comparator == "time" {
		normalized, err := timestamps.Normalize(v)
		if err != nil {
			return false, fmt.Errorf("cursor value %q is not a timestamp: %w", v, err)
		}
		v = normalized
	}
	return t.cmp.Less(v, start)
}

// RecordBelow reports whether a record's cursor sorts before Start. It reads
// the cursor like ObserveChecked, so a numeric epoch cursor compares as time.
// A window-only tracker, or one without a Start, admits every record without
// reading its cursor.
func (t *Tracker) RecordBelow(record map[string]any) (bool, error) {
	if t.windowOnly || t.effective() == "" {
		return false, nil
	}
	v, err := t.Cursor(record)
	if err != nil {
		return false, err
	}
	return t.Below(v)
}

// Scope returns the {start_param: effective_start} pair, plus
// {end_param: upper_bound} when the spec declares one, suitable for merging
// into a template scope's State map. Both bounds are fixed for the entire
// extraction so paginated requests all scan the same result set. The values
// match what Apply injects. Scope is nil when there is no bound to send.
func (t *Tracker) Scope() map[string]string {
	var out map[string]string
	if v := t.injectedValue(); v != "" {
		out = map[string]string{t.spec.StartParam: v}
	}
	if end := t.endValue(); end != "" {
		if out == nil {
			out = map[string]string{}
		}
		out[t.spec.EndParam] = end
	}
	return out
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
	effective := parsed.Add(-time.Duration(t.spec.OverlapSeconds) * time.Second)
	if t.spec.CommitOnComplete && effective.Before(time.Unix(0, 0)) {
		effective = time.Unix(0, 0)
	}
	return effective.Format(time.RFC3339)
}

// Apply injects the extraction's fixed starting watermark, and the end bound
// when end_param is set, into a request. For body-injection, returns a body
// overrides map to be merged before encoding.
// Returns nil overrides for query/header strategies. The injected value
// carries the configured value prefix; the checkpoint keeps the raw value.
func (t *Tracker) Apply(req *http.Request) (map[string]any, error) {
	values := t.Scope()
	if len(values) == 0 {
		return nil, nil
	}
	switch t.spec.InjectInto {
	case "query":
		q := req.URL.Query()
		for param, v := range values {
			q.Set(param, v)
		}
		req.URL.RawQuery = q.Encode()
	case "header":
		for param, v := range values {
			req.Header.Set(param, v)
		}
	case "body":
		out := make(map[string]any, len(values))
		for param, v := range values {
			out[param] = v
		}
		return out, nil
	}
	return nil, nil
}

// endValue returns the upper bound as it must appear on the wire, reformatted
// when value_date_format is set. Empty when the spec declares no end_param.
func (t *Tracker) endValue() string {
	if t.end == "" || t.spec.ValueDateFormat == "" {
		return t.end
	}
	formatted, err := formatWatermark(t.end, t.spec.ValueDateFormat)
	if err != nil {
		t.logger.Warn("incremental: cannot format end bound",
			"resource", t.resource, "value", t.end, "format", t.spec.ValueDateFormat, "error", err)
		return t.end
	}
	return formatted
}

// injectedValue returns the watermark as it must appear on the wire: the
// effective start, reformatted when value_date_format is set, with the
// configured value prefix prepended. The checkpoint stores the raw value
// so ordering comparisons stay prefix- and format-free.
func (t *Tracker) injectedValue() string {
	v := t.effective()
	if v == "" {
		return ""
	}
	if t.spec.ValueDateFormat != "" {
		formatted, err := formatWatermark(v, t.spec.ValueDateFormat)
		if err != nil {
			t.logger.Warn("incremental: cannot format watermark",
				"resource", t.resource, "value", v, "format", t.spec.ValueDateFormat, "error", err)
		} else {
			v = formatted
		}
	}
	return t.spec.ValuePrefix + v + t.spec.ValueSuffix
}

// formatWatermark renders an RFC3339 watermark in the requested wire
// format. "epoch_millis" and "epoch_seconds" render Unix time; anything
// else is a Go time layout.
func formatWatermark(v, format string) (string, error) {
	parsed, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return "", err
	}
	switch format {
	case "epoch_millis":
		return strconv.FormatInt(parsed.UnixMilli(), 10), nil
	case "epoch_seconds":
		return strconv.FormatInt(parsed.Unix(), 10), nil
	default:
		return parsed.Format(format), nil
	}
}
