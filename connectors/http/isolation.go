package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

var _ filament.ResourceIsolatingSource = (*Source)(nil)

// IsolatesResourceFailures reports that a quarantined resource stops alone:
// its reads return quietly on filament.ErrResourceQuarantined and the other
// resources keep reading.
func (*Source) IsolatesResourceFailures() bool { return true }

// statusError is an HTTP error response that survived retries. Its message is
// the one the engine always reported. The status lets isolation classify it.
type statusError struct {
	status int
	msg    string
}

func (e *statusError) Error() string { return e.msg }

// resourceFault marks a failure that concerns only the resource being read:
// a response or record that does not match the manifest, or per-resource
// configuration that cannot address the endpoint. It never wraps a sink or
// transport error.
type resourceFault struct{ err error }

func (e *resourceFault) Error() string { return e.err.Error() }
func (e *resourceFault) Unwrap() error { return e.err }

func resourceScoped(err error) error {
	if err == nil {
		return nil
	}
	return &resourceFault{err: err}
}

// scopedStatus lists the responses that reject one endpoint while the
// account, the credentials and the service stay usable for the others: a
// request the endpoint refuses, a scope or plan the token lacks for it, a
// route that does not exist, or a body it cannot accept.
//
// Everything else is fatal. 401 means the credentials themselves were
// refused, which no other resource survives. 429 and rate-limit rules are
// retried to exhaustion and then describe the account's budget. 5xx is
// retried first and stays fatal: from one resource the engine cannot tell a
// broken endpoint from an outage, and a failed run resumes, while a
// quarantined resource turns an outage into a completed run with missing data.
var scopedStatus = map[int]bool{
	http.StatusBadRequest:                 true,
	http.StatusForbidden:                  true,
	http.StatusNotFound:                   true,
	http.StatusMethodNotAllowed:           true,
	http.StatusNotAcceptable:              true,
	http.StatusConflict:                   true,
	http.StatusGone:                       true,
	http.StatusPreconditionFailed:         true,
	http.StatusUnsupportedMediaType:       true,
	http.StatusUnprocessableEntity:        true,
	http.StatusUnavailableForLegalReasons: true,
}

// isResourceScoped reports whether err leaves every other resource readable.
// An unrecognized error is fatal.
func isResourceScoped(err error) bool {
	var fault *resourceFault
	if errors.As(err, &fault) {
		return true
	}
	var status *statusError
	return errors.As(err, &status) && scopedStatus[status.status]
}

// contain decides what one resource's read error does. Without a reporter
// (strict mode) every error is returned unchanged. Under isolation, a
// resource the engine already quarantined stops quietly, a resource-scoped
// failure is reported once through FailResource when report is set, and
// anything else is returned and fails the run. stopped reports that the
// resource did not finish, so nothing may rely on its captures.
func contain(ctx context.Context, failures filament.ResourceFailureReporter, resource string, report bool, err error) (stopped bool, fatal error) {
	if err == nil {
		return false, nil
	}
	if failures == nil || ctx.Err() != nil {
		return false, err
	}
	if errors.Is(err, filament.ErrResourceQuarantined) {
		return true, nil
	}
	if !isResourceScoped(err) {
		return false, err
	}
	if report {
		failures.FailResource(resource, err)
	}
	return true, nil
}

// extractIsolated is extract under isolation. Every top-level resource runs to
// its own outcome, then children run in dependency order. A resource that did
// not finish (reported or quarantined) leaves a partial capture set, so every
// descendant is reported failed instead of being read from it. Capture-only
// resources are never reported, because the engine has no such resource, but
// their failure still fails their descendants. A nil return means every
// resource that was not reported was read completely.
func (c *Connector) extractIsolated(ctx context.Context, sink recordSink, failures filament.ResourceFailureReporter, topLevel, children []manifest.Resource) error {
	stopped := make(map[string]error)
	var mu sync.Mutex
	outcomes := make([]error, len(topLevel))
	var wg sync.WaitGroup
	for i, res := range topLevel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := c.extractResource(ctx, res, sink, nil)
			halted, fatal := contain(ctx, failures, res.Name, !res.CaptureOnly, err)
			outcomes[i] = fatal
			if halted {
				mu.Lock()
				stopped[res.Name] = err
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	var succeeded int
	var firstErr error
	for _, err := range outcomes {
		if err == nil {
			succeeded++
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("extraction failed (%d/%d resources succeeded): %w", succeeded, len(topLevel), firstErr)
	}

	byName := make(map[string]manifest.Resource, len(c.manifest.Resources))
	for _, res := range c.manifest.Resources {
		byName[res.Name] = res
	}
	for _, res := range manifest.SortResources(children) {
		if ancestor, cause := failedAncestor(res, byName, stopped); cause != nil {
			if res.EmitAs != "" {
				// The engine knows this child only by names rendered from
				// parents that were never read, so it cannot be reported.
				return fmt.Errorf("child resource extraction failed: %s: parent resource %q did not complete: %w", res.Name, ancestor, cause)
			}
			err := fmt.Errorf("parent resource %q did not complete: %w", ancestor, cause)
			if !res.CaptureOnly {
				failures.FailResource(res.Name, err)
			}
			stopped[res.Name] = err
			continue
		}
		if res.Mode != "export" && len(c.snapshotCaptures(res.Parent.Resource)) == 0 {
			continue
		}
		halted, err := c.extractChildIsolated(ctx, res, sink, failures)
		if err != nil {
			return fmt.Errorf("child resource extraction failed: %w", err)
		}
		if halted != nil {
			stopped[res.Name] = halted
		}
	}
	return nil
}

// failedAncestor returns the nearest ancestor of res that did not finish.
func failedAncestor(res manifest.Resource, byName map[string]manifest.Resource, stopped map[string]error) (string, error) {
	for parent := res.Parent; parent != nil; parent = byName[parent.Resource].Parent {
		if cause := stopped[parent.Resource]; cause != nil {
			return parent.Resource, cause
		}
	}
	return "", nil
}

// extractChildIsolated reads one child resource across its parents. A failure
// for one parent fails the whole child: it is reported once, the remaining
// parents are skipped and no other resource is cancelled. A child with
// emit_as is a separate engine resource per parent, so each parent's failure
// is reported under its own name and the other parents keep reading. halted
// is the cause when any part of the child did not finish.
func (c *Connector) extractChildIsolated(ctx context.Context, res manifest.Resource, sink recordSink, failures filament.ResourceFailureReporter) (halted, fatal error) {
	whole := func(err error) (error, error) {
		if res.EmitAs != "" {
			return nil, err
		}
		stop, fatal := contain(ctx, failures, res.Name, !res.CaptureOnly, err)
		if stop {
			return err, nil
		}
		return nil, fatal
	}
	parents := c.snapshotCaptures(res.Parent.Resource)
	parents = filterParentsWithPathParams(res, parents)
	if res.Mode == "export" {
		return whole(c.runExport(ctx, res, sink, parents))
	}
	if res.Parent.Since != "" {
		var err error
		if parents, err = c.gateParents(res, parents); err != nil {
			return whole(err)
		}
	}
	if len(parents) == 0 {
		return nil, nil
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

	// A plain group: one child's contained failure must not cancel the
	// others. fanCtx stops only this child's remaining parents.
	fanCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var g errgroup.Group
	g.SetLimit(concurrency)
	var mu sync.Mutex
	for _, parent := range parents {
		g.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if fanCtx.Err() != nil {
				return nil
			}
			err := c.extractResource(fanCtx, res, sink, parent)
			if err == nil {
				return nil
			}
			err = fmt.Errorf("parent %v: %w", parent, err)
			mu.Lock()
			defer mu.Unlock()
			if res.EmitAs == "" && halted != nil {
				// The child already failed as a whole and this read was cut
				// short with it. Its outcome is decided.
				if ctx.Err() != nil {
					return err
				}
				return nil
			}
			name := res.Name
			if res.EmitAs != "" {
				var nameErr error
				if name, nameErr = emittedResourceName(res, parent, c.env); nameErr != nil {
					return err
				}
			}
			stop, fatal := contain(ctx, failures, name, !res.CaptureOnly, err)
			if fatal != nil {
				return fatal
			}
			if stop {
				if halted == nil {
					halted = err
				}
				if res.EmitAs == "" {
					cancel()
				}
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return halted, nil
}

// failureReporter returns the inlet's reporter when the run isolates resource
// failures, or nil for strict mode.
func failureReporter(sink arrowbatch.Inlet) filament.ResourceFailureReporter {
	failures, _ := sink.(filament.ResourceFailureReporter)
	return failures
}
