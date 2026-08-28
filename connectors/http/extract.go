package httpapi

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/pagination"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Extract runs a full extraction across all enabled resources.
func (c *Connector) Extract(ctx context.Context, sink recordSink, opts extractOptions) error {
	c.observe = opts.Observe
	c.watermarkReported.Clear()
	defer func() {
		c.observe = nil
		c.watermarkReported.Clear()
	}()
	return c.extract(ctx, sink, opts)
}

func (c *Connector) extract(ctx context.Context, sink recordSink, opts extractOptions) error {
	c.resumeStates = opts.ResumeStates
	c.resumeWatermarks = opts.ResumeWatermarks
	c.incrementalLookbacks = opts.IncrementalLookbacks
	c.incrementalResources = opts.IncrementalResources
	c.buildEnabledFilter(opts.EnabledResources)
	c.buildResourceFilter(opts.Resources)

	topLevel, children := manifest.Split(c.filteredResources(c.manifest.Resources))

	outcomes := c.extractConcurrent(ctx, topLevel, sink)

	var succeeded int
	var firstErr error
	for _, err := range outcomes {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			succeeded++
		}
	}
	if firstErr != nil {
		return fmt.Errorf("extraction failed (%d/%d resources succeeded): %w", succeeded, len(topLevel), firstErr)
	}

	sortedChildren := manifest.SortResources(children)
	var childErr error
	for _, res := range sortedChildren {
		if len(c.snapshotCaptures(res.Parent.Resource)) == 0 {
			continue
		}
		if err := c.extractChildResource(ctx, res, sink); err != nil {
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

func (c *Connector) extractConcurrent(ctx context.Context, resources []manifest.Resource, sink recordSink) []error {
	outcomes := make([]error, len(resources))
	var wg sync.WaitGroup

	for i, res := range resources {
		wg.Add(1)
		go func(idx int, res manifest.Resource) {
			defer wg.Done()
			err := c.extractResource(ctx, res, sink, nil)
			outcomes[idx] = err
		}(i, res)
	}

	wg.Wait()
	return outcomes
}

func (c *Connector) extractChildResource(ctx context.Context, res manifest.Resource, sink recordSink) error {
	parents := c.snapshotCaptures(res.Parent.Resource)
	if len(parents) == 0 {
		return nil
	}

	concurrency := res.Parent.Concurrency
	if concurrency <= 0 {
		concurrency = defaultChildConcurrency
	}
	c.observe.Report(filament.SourceProgress{
		Kind:         filament.SourceProgressFanOutStarted,
		Resource:     res.Name,
		ParentsTotal: int64(len(parents)),
	})

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for _, parent := range parents {
		g.Go(func() error {
			err := c.extractResource(ctx, res, sink, parent)
			if err != nil {
				return fmt.Errorf("parent %v: %w", parent, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// extractResource runs one resource end-to-end. Top-level resources may resume
// from the pagination state and watermark supplied by the engine. Child resources always
// restart pagination because a resource-wide cursor cannot be applied to each
// parent independently.
func (c *Connector) extractResource(ctx context.Context, res manifest.Resource, sink recordSink, parent Capture) error {
	pag, err := pagination.New(res.Pagination)
	if err != nil {
		return fmt.Errorf("paginator: %w", err)
	}
	extractor := response.New(res.Response)

	stateResource := res.Name
	if parent != nil && res.EmitAs != "" {
		stateResource, err = emittedResourceName(res, parent)
		if err != nil {
			return fmt.Errorf("resource name: %w", err)
		}
	}
	var tracker *incremental.Tracker
	if res.Incremental != nil && c.incrementalEnabled(stateResource, res.Name) {
		spec := *res.Incremental
		if field, ok := manifest.IncrementalCursorField(res); ok {
			spec.CursorPath = field.Path
		}
		if lookback, ok := c.incrementalLookbacks[stateResource]; ok {
			spec.OverlapSeconds = lookback
		} else if lookback, ok := c.incrementalLookbacks[res.Name]; ok {
			spec.OverlapSeconds = lookback
		}
		seed := ""
		if c.resumeWatermarks != nil {
			seed = c.resumeWatermarks[stateResource][spec.DurableCheckpointKey()]
			if seed == "" && stateResource != res.Name {
				seed = c.resumeWatermarks[res.Name][spec.DurableCheckpointKey()]
			}
		}
		tracker, err = incremental.New(spec, stateResource, seed)
		if err != nil {
			return fmt.Errorf("incremental: %w", err)
		}
	}

	if res.Mode == "stream" {
		_, err := c.streamResource(ctx, res, sink, parent, extractor, tracker)
		return err
	}

	// Cursor resume only applies to top-level resources. Child resources fan
	// out across many parents whose pagination state interleaves into a
	// single resource-level cursor — replaying that cursor for each parent
	// is incorrect, so children always restart pagination from scratch.
	var resumeState pagination.State
	if res.Parent == nil && c.resumeStates != nil {
		resumeState = c.resumeStates[res.Name]
	}

	_, _, err = c.paginate(ctx, res, sink, parent, pag, extractor, tracker, resumeState)
	return err
}

func (c *Connector) incrementalEnabled(resource, base string) bool {
	if c.incrementalResources == nil {
		return true
	}
	return c.incrementalResources[resource] || c.incrementalResources[base]
}

func (c *Connector) reportWatermarkOnce(resource, key, value string) {
	if value == "" {
		return
	}
	if _, loaded := c.watermarkReported.LoadOrStore(resource, struct{}{}); loaded {
		return
	}
	c.observe.Report(filament.SourceProgress{
		Kind:     filament.SourceProgressWatermarkAdvanced,
		Resource: resource,
		Checkpoint: &filament.CheckpointData{
			ResourceName: resource,
			Cursor:       map[string]any{key: value},
		},
	})
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
	required := make(map[string]struct{}, len(c.enabledResources))
	for name := range c.enabledResources {
		required[name] = struct{}{}
	}
	for changed := true; changed; {
		changed = false
		for _, res := range resources {
			if _, ok := required[res.Name]; !ok || res.Parent == nil {
				continue
			}
			if _, ok := required[res.Parent.Resource]; !ok {
				required[res.Parent.Resource] = struct{}{}
				changed = true
			}
		}
	}
	out := make([]manifest.Resource, 0, len(resources))
	for _, res := range resources {
		if _, ok := required[res.Name]; ok {
			out = append(out, res)
		}
	}
	return out
}

// buildEnabledFilter maps caller-supplied EnabledResources (kind+id pairs)
// onto manifest resource names via the Discovery spec, so sendRecords can do
// id-level pushdown filtering by resource name. Empty input clears the
// filter — nil means "extract everything".
func (c *Connector) buildEnabledFilter(refs []resourceRef) {
	c.enabledByResource = nil
	c.enabledIDPath = nil
	if len(refs) == 0 || c.manifest == nil || len(c.manifest.Discovery.Resources) == 0 {
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
	c.enabledByResource = make(map[string]map[string]struct{}, len(c.manifest.Discovery.Resources))
	c.enabledIDPath = make(map[string]string, len(c.manifest.Discovery.Resources))
	for _, d := range c.manifest.Discovery.Resources {
		if set, ok := byKind[d.Map.Kind]; ok {
			c.enabledByResource[d.From] = set
			c.enabledIDPath[d.From] = d.Map.IDPath
		} else if len(anyKind) > 0 {
			c.enabledByResource[d.From] = anyKind
			c.enabledIDPath[d.From] = d.Map.IDPath
		}
	}
}
