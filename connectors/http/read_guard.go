package httpapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

var errReadLimit = errors.New("HTTP resource read limit reached")

// readGuard keeps incomplete, unordered scans from committing a new watermark.
// Only the normal end of a complete scan drains the terminal checkpoint through
// the sink's acknowledgement path. Limited and interrupted reads replay.
type readGuard struct {
	sink         recordSink
	completeOnly bool
	seen         map[string]bool
	count, limit int
	key          []string
}

func (g *readGuard) Push(rec record) error {
	if g.seen != nil {
		if g.seen[rec.ID] {
			return fmt.Errorf("resource %s repeated a primary key during pagination", rec.Resource)
		}
		g.seen[rec.ID] = true
	}
	g.key = rec.Key
	if g.completeOnly {
		rec.Key = nil
	}
	if err := g.sink.Push(rec); err != nil {
		return err
	}
	g.count++
	if g.limit > 0 && g.count >= g.limit {
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

func (g *readGuard) Checkpoint(resource, token string) error {
	return checkpointResponseCursor(g.sink, resource, token)
}

func (c *Connector) extractResource(ctx context.Context, res manifest.Resource, sink recordSink, parent Capture) error {
	for _, field := range res.RequiredConfig {
		if c.creds[field] == "" {
			return fmt.Errorf("%s requires %s", res.Name, field)
		}
	}
	guard := &readGuard{sink: sink, limit: c.limit}
	if res.RestartOnResume && len(res.PrimaryKey) > 0 {
		guard.seen = map[string]bool{}
	}
	guard.completeOnly = res.Incremental != nil && res.Incremental.CommitOnComplete && c.incrementalEnabled(res.Name, res.Name)
	err := c.readResource(ctx, res, guard, parent)
	if errors.Is(err, errReadLimit) {
		return nil
	}
	if err != nil {
		return err
	}
	if guard.completeOnly && guard.count > 0 && len(guard.key) > 0 {
		return checkpointResponseCursor(sink, res.Name, guard.key[0])
	}
	return nil
}
