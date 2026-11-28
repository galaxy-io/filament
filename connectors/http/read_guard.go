package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament/connectors/http/internal/atomicwatermark"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

var errReadLimit = errors.New("HTTP resource read limit reached")

// readGuard keeps incomplete, unordered scans from committing a new watermark.
// Only the normal end of a complete scan drains the terminal checkpoint through
// the sink's acknowledgement path. Limited and interrupted reads replay.
//
// One guard may be shared across the concurrent parents of a fan-out child:
// Push is safe for concurrent use, and the drained terminal key is the
// maximum observed across all parents, not the last record pushed.
type readGuard struct {
	mu           sync.Mutex
	sink         recordSink
	completeOnly bool
	seen         map[string]bool
	count, limit int
	key          []string
	limited      bool
	mark         *atomicwatermark.Watermark
}

// newReadGuard builds a guard for one resource read. When completeOnly is
// set, the guard strips per-batch keys and tracks the maximum key observed,
// so drain promotes only the terminal watermark of a complete scan.
func newReadGuard(sink recordSink, limit int, res manifest.Resource, completeOnly bool) (*readGuard, error) {
	guard := &readGuard{sink: sink, limit: limit, completeOnly: completeOnly}
	if res.RestartOnResume && len(res.PrimaryKey) > 0 {
		guard.seen = map[string]bool{}
	}
	if completeOnly {
		cmp, err := atomicwatermark.ForName(res.Incremental.Comparator)
		if err != nil {
			return nil, err
		}
		mark, err := atomicwatermark.New(cmp, "")
		if err != nil {
			return nil, err
		}
		guard.mark = mark
	}
	return guard, nil
}

func (g *readGuard) Push(rec record) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.seen != nil {
		if g.seen[rec.ID] {
			return resourceScoped(fmt.Errorf("resource %s repeated a primary key during pagination", rec.Resource))
		}
		g.seen[rec.ID] = true
	}
	if g.mark != nil {
		if value := firstKey(rec.Key); value != "" {
			if _, err := g.mark.Observe(value); err != nil {
				return fmt.Errorf("incremental %q watermark: %w", rec.Resource, err)
			}
		}
		g.key = watermarkKey(g.mark.Current())
	} else {
		g.key = rec.Key
	}
	if g.completeOnly {
		rec.Key = nil
	}
	if err := g.sink.Push(rec); err != nil {
		return err
	}
	g.count++
	if g.limit > 0 && g.count >= g.limit {
		g.limited = true
		return errReadLimit
	}
	return nil
}

func (g *readGuard) PushBatch(records []record) error {
	for _, rec := range records {
		if err := g.Push(rec); err != nil {
			return err
		}
	}
	return nil
}

// drain promotes the terminal watermark once, after a complete scan. It is a
// no-op unless the guard is complete-only, at least one record was pushed,
// a key was observed, and the read was not cut short by the limit.
func (g *readGuard) drain(resource string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.completeOnly || g.count == 0 || len(g.key) == 0 || g.limited {
		return nil
	}
	return checkpointResponseCursor(g.sink, resource, g.key[0])
}

// markLimited records that an ancestor read stopped at the row limit, so
// drain stays a no-op and the watermark is left for the next run to replay.
func (g *readGuard) markLimited() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.limited = true
}

func (g *readGuard) Checkpoint(resource, token string) error {
	return checkpointResponseCursor(g.sink, resource, token)
}

func (c *Connector) extractResource(ctx context.Context, res manifest.Resource, sink recordSink, parent Capture) error {
	return c.extractResourceWithGuard(ctx, res, sink, parent, nil)
}

// extractResourceWithGuard runs one resource end-to-end. A nil guard builds
// a fresh guard for this read and drains its terminal key afterwards. A
// shared guard is drained once by the caller, after every parent finishes,
// so a fan-out child only promotes its watermark on a complete scan.
func (c *Connector) extractResourceWithGuard(ctx context.Context, res manifest.Resource, sink recordSink, parent Capture, guard *readGuard) error {
	for _, field := range res.RequiredConfig {
		if c.creds[field] == "" {
			return resourceScoped(fmt.Errorf("%s requires %s", res.Name, field))
		}
	}
	shared := guard != nil
	if !shared {
		var err error
		guard, err = newReadGuard(sink, c.limit, res, res.Incremental != nil && res.Incremental.CommitOnComplete && c.incrementalEnabled(res.Name, res.Name))
		if err != nil {
			return err
		}
	}
	err := c.readResource(ctx, res, guard, parent)
	if errors.Is(err, errReadLimit) {
		// The limit is swallowed as success, but the read is incomplete.
		// Record it so a fan-out child of this resource does not promote
		// its watermark from the partial parent set.
		c.markLimited(res.Name)
		return nil
	}
	if err != nil {
		return err
	}
	if shared {
		return nil
	}
	return guard.drain(res.Name)
}
