package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/request"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// The coordinator is the only publisher of a resource's combined job map.
// Rows carry no cursor: an interleaved row must not overwrite a newer job snapshot.
type exportCoordinator struct {
	mu       sync.Mutex
	state    RunState
	target   Sink
	resource string
	dirty    bool
}

func (g *exportCoordinator) publish() error {
	token, err := g.state.token()
	if err != nil {
		return err
	}
	if err := g.target.Checkpoint(g.resource, token); err != nil {
		return err
	}
	g.dirty = false
	return nil
}

func (g *exportCoordinator) flush() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.dirty {
		return nil
	}
	return g.publish()
}

func (g *exportCoordinator) update(key string, job exportScopedJob) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	old := g.state.Parents[key]
	if old.Job != nil && old.Job.Phase == job.Phase && old.Job.Deadline.Equal(job.Deadline) &&
		old.Job.Candidate == job.Candidate && maps.Equal(old.Job.Captures, job.Captures) {
		return nil
	}
	next := old
	copyJob := job
	copyJob.Captures = maps.Clone(job.Captures)
	next.Job = &copyJob
	g.state.Parents[key] = next
	wasDirty := g.dirty
	g.dirty = true
	// Completion is safe to replay. Fold it into the next identity checkpoint
	// or the final flush, rather than serializing every parent again here.
	if job.Phase == "done" {
		return nil
	}
	if err := g.publish(); err != nil {
		g.state.Parents[key] = old
		g.dirty = wasDirty
		return err
	}
	return nil
}

func (c *Runtime) runScoped(ctx context.Context, res manifest.Resource, sink Sink, parents []map[string]string) error {
	target := sink
	inc := res.Incremental != nil && c.Incremental
	state, err := c.decodeExportRun(res, c.ResumeToken, inc)
	if err != nil {
		return err
	}
	if !state.Initialized {
		if res.Parent == nil {
			parents = []map[string]string{nil}
		}
		if err := c.initializeExportParents(res, &state, parents); err != nil {
			return err
		}
	}
	// Recovery must replay even completed jobs: the last transaction may have
	// rolled back. A fresh scheduled plan clears completed jobs before this point.
	for _, key := range state.Selected {
		entry := state.Parents[key]
		if entry.Job.Phase == "creating" {
			return fmt.Errorf("export %s: creation outcome for a parent is unknown; reconcile before a fresh read", res.Name)
		}
		if entry.Job.Phase == "done" {
			entry.Job.Phase = "waiting"
			entry.Job.Candidate = ""
		}
	}
	group := &exportCoordinator{state: state, target: target, resource: res.Name}
	if err := group.publish(); err != nil {
		return err
	}
	concurrency := 1
	if res.Parent != nil {
		concurrency = res.Parent.Concurrency
		if concurrency <= 0 {
			concurrency = 5
		}
	}
	workers, ctx := errgroup.WithContext(ctx)
	workers.SetLimit(concurrency)
	// Copy the work list before workers can mutate the coordinator's map.
	jobs := make([]exportParentState, len(state.Selected))
	for i, key := range state.Selected {
		jobs[i] = state.Parents[key]
	}
	for i, key := range state.Selected {
		entry := jobs[i]
		workers.Go(func() error { return c.runScopedExport(ctx, res, sink, group, key, entry) })
	}
	err = workers.Wait()
	// Flush even on failure so completed candidates and the last captured job
	// identities survive a checkpointable pause. This never promotes watermarks.
	return errors.Join(err, group.flush())
}

func (c *Runtime) initializeExportParents(res manifest.Resource, state *RunState, parents []map[string]string) error {
	if len(parents) > maxExportParents {
		return fmt.Errorf("export %s exceeds %d parents", res.Name, maxExportParents)
	}
	selected := make(map[string]bool, len(parents))
	for _, parent := range parents {
		key, err := exportParentKey(res, parent)
		if err != nil {
			return err
		}
		if selected[key] {
			return fmt.Errorf("export %s has duplicate parent keys", res.Name)
		}
		selected[key] = true
		entry := state.Parents[key]
		entry.Parent = maps.Clone(parent)
		overlap := 0
		if state.Incremental {
			overlap = res.Incremental.OverlapSeconds
			if c.Lookback != nil {
				overlap = *c.Lookback
			}
		}
		tracker, err := exportTracker(res, state.Incremental, entry.Committed, overlap)
		if err != nil {
			return err
		}
		seed := ""
		if tracker != nil {
			seed = tracker.Current()
		}
		entry.Job = &exportScopedJob{Phase: "planned", Seed: seed, Overlap: overlap}
		state.Parents[key] = entry
		state.Selected = append(state.Selected, key)
	}
	if len(state.Parents) > maxExportParents {
		return fmt.Errorf("export parent history exceeds %d entries", maxExportParents)
	}
	slices.Sort(state.Selected)
	state.Initialized = true
	return nil
}

func exportParentKey(res manifest.Resource, parent map[string]string) (string, error) {
	if res.Parent == nil {
		return "$", nil
	}
	values := make([]string, len(res.Export.ParentKey))
	for i, key := range res.Export.ParentKey {
		if parent[key] == "" {
			return "", fmt.Errorf("export %s: missing parent key %q", res.Name, key)
		}
		values[i] = parent[key]
	}
	raw, _ := json.Marshal(values)
	return string(raw), nil
}

func exportTracker(res manifest.Resource, enabled bool, seed string, overlap int) (*incremental.Tracker, error) {
	if !enabled {
		return nil, nil
	}
	spec := *res.Incremental
	field, ok := manifest.IncrementalCursorField(res)
	if !ok {
		return nil, fmt.Errorf("export incremental cursor field is missing")
	}
	spec.CursorPath = field.Path
	spec.OverlapSeconds = overlap
	return incremental.New(spec, res.Name, seed)
}

// scopedExportStart builds a send-time request from the validated manifest and
// frozen recovery inputs. The rendered request is never checkpointed.
func (c *Runtime) scopedExportStart(res manifest.Resource, parent map[string]string, tracker *incremental.Tracker) (manifest.Resource, error) {
	rendered, err := request.RenderResource(res.Export.Start.Resource(res.Name), c.scopeFor(parent, tracker))
	if err != nil {
		return manifest.Resource{}, err
	}
	if tracker != nil {
		req := &http.Request{URL: &url.URL{}, Header: make(http.Header)}
		body, err := tracker.Apply(req)
		if err != nil {
			return manifest.Resource{}, err
		}
		if len(body) > 0 {
			rendered.Body.Template = request.MergeOverrides(rendered.Body.Template, nil, body)
		}
		for key, values := range req.URL.Query() {
			if rendered.Query == nil {
				rendered.Query = map[string]string{}
			}
			rendered.Query[key] = values[0]
		}
		for key, values := range req.Header {
			if rendered.Headers == nil {
				rendered.Headers = map[string]string{}
			}
			rendered.Headers[key] = values[0]
		}
	}
	return rendered, nil
}

func (c *Runtime) runScopedExport(ctx context.Context, res manifest.Resource, sink Sink, group *exportCoordinator, key string, entry exportParentState) error {
	job := *entry.Job
	job.Captures = maps.Clone(job.Captures)
	progress := c.jobProgress(res.Name, job.Phase == "waiting")
	tracker, err := exportTracker(res, group.state.Incremental, job.Seed, job.Overlap)
	if err != nil {
		return err
	}
	scope := c.scopeFor(entry.Parent, tracker)
	if job.Phase == "planned" {
		job.Phase = "creating"
		job.Deadline = time.Now().Add(time.Duration(res.Export.Wait.TimeoutSeconds) * time.Second)
	}
	// Renew an expired waiting budget only at a new execution boundary. A
	// completed job converted to waiting for replay needs the same treatment.
	if job.Phase == "waiting" && !time.Now().Before(job.Deadline) {
		job.Deadline = time.Now().Add(time.Duration(res.Export.Wait.TimeoutSeconds) * time.Second)
		if err := group.update(key, job); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithDeadline(ctx, job.Deadline)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if job.Phase == "creating" {
		start, err := c.scopedExportStart(res, entry.Parent, tracker)
		if err != nil {
			return err
		}
		if err := group.update(key, job); err != nil {
			return err
		}
		raw, err := c.Control.Start(ctx, start, scope)
		if err != nil {
			return fmt.Errorf("export %s creation outcome requires reconciliation: %w", res.Name, err)
		}
		job.Captures = map[string]string{}
		if err := captureExport(raw, res.Export.Start.Capture, job.Captures); err != nil {
			return err
		}
		job.Phase = "waiting"
		progress.report(filament.SourceProgressExportJobCreated)
		if err := group.update(key, job); err != nil {
			return err
		}
	}
	scope.Job = job.Captures
	if res.Export.Wait.Type == "job" {
		if err := c.waitExport(ctx, res, scope, progress); err != nil {
			return err
		}
		if err := group.update(key, job); err != nil {
			return err
		}
	}
	count, downloadBytes, err := c.consumeScopedExport(ctx, res, sink, entry.Parent, scope, tracker, progress)
	if err != nil {
		return err
	}
	job.Phase = "done"
	if tracker != nil {
		job.Candidate = tracker.Current()
	}
	if err := group.update(key, job); err != nil {
		return err
	}
	sink.Completed(count, downloadBytes)
	return nil
}

func (c *Runtime) consumeScopedExport(ctx context.Context, res manifest.Resource, sink Sink, parent map[string]string, scope template.Scope, tracker *incremental.Tracker, progress *jobProgress) (int, int64, error) {
	location, err := template.Render(res.Export.Result.URL, scope)
	if err != nil {
		return 0, 0, err
	}
	resp, err := c.downloadExport(ctx, res, location, scope, progress)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	count := 0
	artifact := &artifactReader{Reader: resp.Body}
	err = decodeExport(ctx, artifact, res.Export.Result, func(row map[string]any) error {
		if tracker != nil {
			if _, err := tracker.ObserveChecked(row); err != nil {
				return err
			}
		}
		n, err := sink.Emit(row, parent, "")
		count += n
		return err
	})
	if err == nil {
		err = ctx.Err()
	}
	return count, artifact.bytes, err
}
