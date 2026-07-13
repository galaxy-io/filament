package httpapi

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline/integrity"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
	"github.com/galaxy-io/filament/connectors/http/pagination"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Extract runs a full extraction across all enabled resources.
func (c *Connector) Extract(ctx context.Context, opts pipeline.ExtractOptions) error {
	return c.extract(ctx, opts, nil)
}

// ExtractFrom resumes an extraction from a previous checkpoint, skipping
// resources it records as complete.
func (c *Connector) ExtractFrom(ctx context.Context, opts pipeline.ExtractOptions, prev *integrity.PipelineCheckpoint) error {
	return c.extract(ctx, opts, prev)
}

type resourceOutcome struct {
	Resource string
	Err      error
}

func (c *Connector) extract(ctx context.Context, opts pipeline.ExtractOptions, prev *integrity.PipelineCheckpoint) error {
	c.logger = obs.Logger(opts.Logger)
	c.reporter = obs.Reporter(opts.Reporter)
	c.commitCheckpoint = opts.Checkpoint
	c.resumeCursors = opts.ResumeCursors
	c.resumeWatermarks = opts.ResumeWatermarks
	c.buildEnabledFilter(opts.EnabledResources)
	c.buildResourceFilter(opts.Resources)

	sink := opts.Sink

	topLevel, children := manifest.Split(c.filteredResources(c.manifest.Resources))

	pending := topLevel
	if prev != nil {
		pending = pending[:0]
		for _, res := range topLevel {
			if prev.IsResourceComplete(res.Name) {
				// Rehydrate captured parent records from the stored
				// checkpoint so child resources can still fan out across
				// them on resume (otherwise the child silently no-ops).
				//
				// Validate persisted shape against the current capture spec
				// first — manifests can change between runs, and a stale
				// checkpoint with mismatched keys would silently template
				// child requests with wrong/missing fields.
				if err := c.rehydrateCaptures(res, prev); err != nil {
					c.logger.Warn("checkpoint captures stale, re-extracting resource",
						"resource", res.Name, "error", err)
					pending = append(pending, res)
					continue
				}
				c.logger.Info("skipping completed resource", "resource", res.Name)
				continue
			}
			pending = append(pending, res)
		}
	}

	outcomes := c.extractConcurrent(ctx, pending, sink, prev)

	var succeeded int
	var firstErr error
	for _, o := range outcomes {
		if o.Err != nil {
			c.reporter.Report(pipeline.Event{
				Type:      pipeline.EventResourceFailed,
				Resource:  o.Resource,
				Connector: c.manifest.Name,
				Error:     o.Err,
			})
			if firstErr == nil {
				firstErr = o.Err
			}
		} else {
			succeeded++
		}
	}
	if firstErr != nil {
		return fmt.Errorf("extraction failed (%d/%d resources succeeded): %w", succeeded, len(pending), firstErr)
	}

	sortedChildren := manifest.SortResources(children)
	var childErr error
	for _, res := range sortedChildren {
		if prev != nil && prev.IsResourceComplete(res.Name) {
			continue
		}
		if len(c.snapshotCaptures(res.Parent.Resource)) == 0 {
			continue
		}
		if err := c.extractChildResource(ctx, res, sink, prev); err != nil {
			c.reporter.Report(pipeline.Event{
				Type:      pipeline.EventResourceFailed,
				Resource:  res.Name,
				Connector: c.manifest.Name,
				Error:     err,
			})
			if childErr == nil {
				childErr = err
			}
		}
	}
	if childErr != nil {
		return fmt.Errorf("child resource extraction failed: %w", childErr)
	}
	return nil
}

func (c *Connector) extractConcurrent(ctx context.Context, resources []manifest.Resource, sink *pipeline.RecordSink, prev *integrity.PipelineCheckpoint) []resourceOutcome {
	outcomes := make([]resourceOutcome, len(resources))
	var wg sync.WaitGroup

	for i, res := range resources {
		wg.Add(1)
		go func(idx int, res manifest.Resource) {
			defer wg.Done()
			c.reporter.Report(pipeline.Event{
				Type:      pipeline.EventResourceStart,
				Resource:  res.Name,
				Connector: c.manifest.Name,
			})

			n, pages, err := c.extractResource(ctx, res, sink, nil, prev)
			if err == nil {
				c.reporter.Report(pipeline.Event{
					Type:         pipeline.EventResourceComplete,
					Resource:     res.Name,
					Connector:    c.manifest.Name,
					TotalRecords: n,
					Pages:        pages,
				})
			}
			outcomes[idx] = resourceOutcome{Resource: res.Name, Err: err}
		}(i, res)
	}

	wg.Wait()
	return outcomes
}

func (c *Connector) extractChildResource(ctx context.Context, res manifest.Resource, sink *pipeline.RecordSink, prev *integrity.PipelineCheckpoint) error {
	parents := c.snapshotCaptures(res.Parent.Resource)
	if len(parents) == 0 {
		return nil
	}

	concurrency := res.Parent.Concurrency
	if concurrency <= 0 {
		concurrency = defaultChildConcurrency
	}

	c.reporter.Report(pipeline.Event{
		Type:         pipeline.EventResourceStart,
		Resource:     res.Name,
		Connector:    c.manifest.Name,
		ParentsTotal: len(parents),
	})
	c.reporter.Report(pipeline.Event{
		Type:         pipeline.EventFanOutStart,
		Resource:     res.Name,
		Connector:    c.manifest.Name,
		ParentsTotal: len(parents),
	})

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	var totalRecords atomic.Int64
	var parentsDone atomic.Int64

	for _, parent := range parents {
		g.Go(func() error {
			n, _, err := c.extractResource(ctx, res, sink, parent, prev)
			if err != nil {
				return fmt.Errorf("parent %v: %w", parent, err)
			}
			totalRecords.Add(int64(n))
			parentsDone.Add(1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	c.reporter.Report(pipeline.Event{
		Type:         pipeline.EventResourceComplete,
		Resource:     res.Name,
		Connector:    c.manifest.Name,
		TotalRecords: int(totalRecords.Load()),
		ParentsDone:  int(parentsDone.Load()),
		ParentsTotal: len(parents),
	})
	return nil
}

// extractResource runs one resource end-to-end.
//
// # Nil-guard invariants
//
//   - parent == nil   → top-level resource. Watermark commit + capture
//     persistence both fire. Cursor resume is honoured when prev != nil.
//   - parent != nil   → child resource fanned out across that parent. No
//     watermark commit (children don't own watermarks). No cursor resume
//     (a resource-wide cursor can't be re-applied per-parent — see
//     LastVerifiedCursor godoc).
//   - prev == nil     → non-resumable run. No checkpoint reads, no
//     captures rehydration.
//   - res.Incremental == nil → no tracker is built; commitWatermark is a no-op.
func (c *Connector) extractResource(ctx context.Context, res manifest.Resource, sink *pipeline.RecordSink, parent Capture, prev *integrity.PipelineCheckpoint) (int, int, error) {
	pag, err := pagination.New(res.Pagination)
	if err != nil {
		return 0, 0, fmt.Errorf("paginator: %w", err)
	}
	extractor := response.New(res.Response)

	var tracker *incremental.Tracker
	if res.Incremental != nil {
		seed := incremental.LoadFrom(prev, res.Name, *res.Incremental)
		if seed == "" && c.resumeWatermarks != nil {
			seed = c.resumeWatermarks[res.Name][incremental.CheckpointKey(*res.Incremental)]
		}
		tracker, err = incremental.New(*res.Incremental, res.Name, seed)
		if err != nil {
			return 0, 0, fmt.Errorf("incremental: %w", err)
		}
	}

	if res.Mode == "stream" {
		n, err := c.streamResource(ctx, res, sink, parent, extractor, tracker)
		if err == nil {
			if cerr := c.commitWatermark(tracker); cerr != nil {
				return n, 0, cerr
			}
			c.persistCapturesIfTopLevel(res, parent)
		}
		return n, 0, err
	}

	// Cursor resume only applies to top-level resources. Child resources fan
	// out across many parents whose pagination state interleaves into a
	// single resource-level cursor — replaying that cursor for each parent
	// is incorrect, so children always restart pagination from scratch.
	startCursor := ""
	if prev != nil && res.Parent == nil {
		startCursor = prev.LastVerifiedCursor(res.Name)
	} else if res.Parent == nil && c.resumeCursors != nil {
		startCursor = c.resumeCursors[res.Name]
	}

	n, pages, err := c.paginate(ctx, res, sink, parent, pag, extractor, tracker, startCursor)
	if err == nil {
		if cerr := c.commitWatermark(tracker); cerr != nil {
			return n, pages, cerr
		}
		c.persistCapturesIfTopLevel(res, parent)
	}
	return n, pages, err
}

// persistCapturesIfTopLevel writes the captured parent records to the runner-
// supplied checkpoint so that child resources can fan out across already-
// completed parents on resume. No-op for child resources, no-op when no
// checkpoint is supplied.
func (c *Connector) persistCapturesIfTopLevel(res manifest.Resource, parent Capture) {
	if parent != nil || c.commitCheckpoint == nil {
		return
	}
	captured := c.snapshotCaptures(res.Name)
	if len(captured) == 0 {
		return
	}
	c.commitCheckpoint.AppendCaptures(res.Name, captured)
}

// commitWatermark persists the tracker's current watermark to the runner-
// supplied checkpoint. No-op when no tracker, no checkpoint, or no advance.
//
// Commit failures (corrupt comparator state, unparseable existing checkpoint
// value) are surfaced as an error rather than warn-logged: silently
// preferring the new value would advance past whatever real watermark the
// checkpoint held, breaking at-least-once semantics on the next run.
func (c *Connector) commitWatermark(tracker *incremental.Tracker) error {
	if tracker == nil || c.commitCheckpoint == nil {
		return nil
	}
	return tracker.Commit(c.commitCheckpoint)
}

// scopeFor builds the per-request template scope.
func (c *Connector) scopeFor(parent Capture, cursor string, tracker *incremental.Tracker) template.Scope {
	scope := template.Scope{
		Config: c.creds,
		Parent: parent,
		Cursor: cursor,
	}
	if tracker != nil {
		scope.State = tracker.Scope()
	}
	return scope
}

// reportWatermarkOnce emits EventWatermarkAdvanced the first time the
// tracker advances on this resource during the current extraction. Further
// advances are silent — one event per resource conveys "making incremental
// progress" without flooding the stream.
func (c *Connector) reportWatermarkOnce(resource string, tracker *incremental.Tracker) {
	if tracker == nil {
		return
	}
	if _, loaded := c.watermarkReported.LoadOrStore(resource, struct{}{}); loaded {
		return
	}
	c.reporter.Report(pipeline.Event{
		Type:      pipeline.EventWatermarkAdvanced,
		Resource:  resource,
		Connector: c.manifest.Name,
		Cursor:    tracker.Current(),
		Field:     tracker.CursorField(),
	})
}

// snapshotCaptures returns a defensive copy of the captured parent records
// for a resource. The caller owns the returned slice header, so concurrent
// appendCaptures calls cannot race against iteration of the result.
//
// Note: the inner Capture maps are NOT deep-copied. Captures are written once
// per record at capture-time and never mutated afterwards (the connector
// treats them as immutable), so sharing references is safe. Tests assert this
// contract — see TestSnapshotCaptures_ReturnsIndependentCopy.
func (c *Connector) snapshotCaptures(name string) []Capture {
	c.mu.Lock()
	defer c.mu.Unlock()
	src := c.parentRecords[name]
	if len(src) == 0 {
		return nil
	}
	out := make([]Capture, len(src))
	copy(out, src)
	return out
}

// appendCaptures atomically appends captured parent fields for a resource.
// Safe for concurrent calls from fan-out goroutines (parent.go's errgroup
// fan-out, NDJSON streamReader's emit).
func (c *Connector) appendCaptures(name string, captured []Capture) {
	if len(captured) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.parentRecords[name] = append(c.parentRecords[name], captured...)
}

// rehydrateCaptures pulls a previously-completed resource's captured parent
// records from the checkpoint into in-memory parentRecords so child fan-out
// works on resume.
//
// Validates that every persisted capture map carries the same key set as the
// current res.Capture spec. A mismatch means the manifest changed (or the
// checkpoint is from a different connector version) and silently re-using the
// stale data would template child requests with wrong fields. The caller
// treats a non-nil error as "re-extract this resource".
//
// Returns nil (no-op) when the resource declares no Capture block — there's
// nothing for children to consume so persisted captures are immaterial.
func (c *Connector) rehydrateCaptures(res manifest.Resource, prev *integrity.PipelineCheckpoint) error {
	if len(res.Capture) == 0 {
		return nil
	}
	persisted := prev.GetCaptures(res.Name)
	if len(persisted) == 0 {
		return nil
	}
	if err := validateCaptureShape(res.Capture, persisted); err != nil {
		return err
	}
	c.appendCaptures(res.Name, persisted)
	return nil
}

func (c *Connector) buildResourceFilter(resources []string) {
	c.enabledResources = nil
	if len(resources) == 0 {
		return
	}
	c.enabledResources = make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if resource == "" {
			continue
		}
		c.enabledResources[resource] = struct{}{}
	}
}

func (c *Connector) filteredResources(resources []manifest.Resource) []manifest.Resource {
	if len(c.enabledResources) == 0 {
		return resources
	}
	out := make([]manifest.Resource, 0, len(resources))
	for _, res := range resources {
		if _, ok := c.enabledResources[res.Name]; ok {
			out = append(out, res)
		}
	}
	return out
}

// buildEnabledFilter maps caller-supplied EnabledResources (kind+id pairs)
// onto manifest resource names via the Discovery spec, so sendRecords can do
// id-level pushdown filtering by resource name. Empty input clears the
// filter — nil means "extract everything".
func (c *Connector) buildEnabledFilter(refs []pipeline.ResourceRef) {
	c.enabledByResource = nil
	c.enabledIDPath = nil
	if len(refs) == 0 || c.manifest == nil || len(c.manifest.Discovery) == 0 {
		return
	}
	byKind := make(map[string]map[string]struct{})
	anyKind := make(map[string]struct{})
	for _, r := range refs {
		if r.Kind == "" {
			anyKind[r.ID] = struct{}{}
			continue
		}
		set, ok := byKind[r.Kind]
		if !ok {
			set = make(map[string]struct{})
			byKind[r.Kind] = set
		}
		set[r.ID] = struct{}{}
	}
	c.enabledByResource = make(map[string]map[string]struct{}, len(c.manifest.Discovery))
	c.enabledIDPath = make(map[string]string, len(c.manifest.Discovery))
	for _, d := range c.manifest.Discovery {
		if set, ok := byKind[d.Map.Kind]; ok {
			c.enabledByResource[d.From] = set
			c.enabledIDPath[d.From] = d.Map.IDPath
		} else if len(anyKind) > 0 {
			c.enabledByResource[d.From] = anyKind
			c.enabledIDPath[d.From] = d.Map.IDPath
		}
	}
}

// validateCaptureShape errors when any persisted capture map's key set does
// not match the resource's current Capture spec. Catches a manifest change
// between runs (added/removed/renamed capture fields) that would otherwise
// silently propagate stale field shapes into child requests.
func validateCaptureShape(spec map[string]string, persisted []Capture) error {
	expected := make(map[string]struct{}, len(spec))
	for k := range spec {
		expected[k] = struct{}{}
	}
	for i, cap := range persisted {
		if len(cap) != len(expected) {
			return fmt.Errorf("capture[%d]: persisted has %d fields, spec declares %d",
				i, len(cap), len(expected))
		}
		for k := range cap {
			if _, ok := expected[k]; !ok {
				return fmt.Errorf("capture[%d]: persisted field %q not in current spec", i, k)
			}
		}
	}
	return nil
}
