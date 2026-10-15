package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"

	"github.com/galaxy-io/filament/connectors/http/internal/export"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Creation is attempted once, including for 202 responses. Redirects cannot
// replay the request; an ambiguous result requires upstream reconciliation.
func (c *Connector) startExport(ctx context.Context, start manifest.Resource, scope template.Scope) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := c.builder.BuildRendered(ctx, start, scope)
	if err != nil {
		return nil, err
	}
	client := *c.client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, export.TransportError(ctx, err)
	}
	defer func() { _ = resp.Body.Close() }()
	c.limiter.Observe(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("creation returned HTTP %d (not retried)", resp.StatusCode)
	}
	return readExportControl(resp.Body)
}

func readExportControl(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, 1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read job response: %w", err)
	}
	if len(b) > 1024*1024 || !gjson.ValidBytes(b) {
		return nil, fmt.Errorf("job response must be valid JSON no larger than 1 MiB")
	}
	return b, nil
}

// Poll retries only transient HTTP errors. The export runtime owns pending-job
// polling. Keep response bodies and URLs out of errors on this control path.
func (c *Connector) pollExport(ctx context.Context, name string, r manifest.ExportRequest, scope template.Scope) ([]byte, error) {
	raw, err := c.getExportControl(ctx, name, "poll", r, scope)
	if err != nil {
		return nil, err
	}
	if len(raw) > 1024*1024 || !gjson.ValidBytes(raw) {
		return nil, fmt.Errorf("export %s: invalid job response", name)
	}
	return raw, nil
}

// locateExport reads a direct export's plain-text locator with the poll
// retry policy. The body is not JSON and is never included in errors.
func (c *Connector) locateExport(ctx context.Context, name string, r manifest.ExportRequest, scope template.Scope) ([]byte, error) {
	raw, err := c.getExportControl(ctx, name, "locator", r, scope)
	if err != nil {
		return nil, err
	}
	if len(raw) > 1024*1024 {
		return nil, fmt.Errorf("export %s: locator response is larger than 1 MiB", name)
	}
	return raw, nil
}

// getExportControl returns at most 1 MiB plus one byte so callers can reject
// oversized bodies.
func (c *Connector) getExportControl(ctx context.Context, name, step string, r manifest.ExportRequest, scope template.Scope) ([]byte, error) {
	client := *c.client
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := c.builder.Build(ctx, r.Resource(name), scope)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, export.TransportError(ctx, err)
		}
		c.limiter.Observe(resp)
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
		_ = resp.Body.Close()
		delay, limited := rateLimitDelay(resp, raw, attempt, c.manifest.Connection.RateLimit)
		if limited || resp.StatusCode >= 500 {
			if !limited {
				delay = retryAfterDuration(resp, attempt)
			}
			if attempt+1 == maxRetries {
				break
			}
			if err := sleepCtx(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("export %s %s returned HTTP %d", name, step, resp.StatusCode)
		}
		if readErr != nil {
			return nil, fmt.Errorf("export %s: reading %s response failed", name, step)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("export %s: %s retries exhausted", name, step)
}
