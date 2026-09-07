package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/pagination"
	"github.com/galaxy-io/filament/connectors/http/request"
	"github.com/galaxy-io/filament/connectors/http/response"
)

// paginate drives the request → response → records → next-state loop.
// Returns (totalRecords, pageCount, err).
//
// # Nil-guard invariants
//
//   - pag == nil  → resource declares no pagination (manifest.PaginationSpec.Type == "").
//     A single request is issued, records flushed, and the loop returns.
//     Apply/Next are never called; this is a normal control-flow path, not
//     an error fallback.
//   - tracker == nil  → resource declares no incremental block. Watermark
//     observation and Apply are skipped.
//   - parent == nil  → top-level resource (no parent fan-out).
//   - resumeState is zero → fresh extraction (no pagination resume).
//
// Any combination of the above is valid. The for-loop never panics on a nil
// pag because every pag-touching call site checks first.
func (c *Connector) paginate(
	ctx context.Context,
	res manifest.Resource,
	sink recordSink,
	parent Capture,
	pag pagination.Paginator,
	extractor *response.Extractor,
	tracker *incremental.Tracker,
	resumeState pagination.State,
) (int, int, error) {
	// pag == nil means the resource declares no pagination — issue one
	// request, process it, return. No Apply/Next calls happen because there
	// is no strategy object to invoke; the fetchPage path nil-guards Apply.
	var state pagination.State
	if pag != nil {
		state = pag.Initial()
		if resumeState != (pagination.State{}) {
			state = resumeState
			c.logger.Info("resuming from pagination checkpoint", "resource", res.Name)
		}
	}

	var totalRecords, pageCount int
	progressResource, err := emittedResourceName(res, parent, c.env)
	if err != nil {
		return 0, 0, fmt.Errorf("resource name: %w", err)
	}

	for {
		resp, raw, err := c.fetchPage(ctx, res, parent, state, pag, tracker)
		if err != nil {
			// Stale-cursor fallback: if we resumed and the first request fails,
			// retry from scratch.
			if pag != nil && resumeState != (pagination.State{}) && pageCount == 0 {
				c.logger.Warn("stale cursor, falling back to full extraction",
					"resource", res.Name, "error", err)
				state = pag.Initial()
				resumeState = pagination.State{}
				continue
			}
			return totalRecords, pageCount, err
		}

		if err := extractor.CheckError(raw); err != nil {
			return totalRecords, pageCount, fmt.Errorf("response error on %s: %w", res.Name, err)
		}

		records, err := extractor.Records(raw)
		if err != nil {
			return totalRecords, pageCount, fmt.Errorf("decode %s: %w", res.Name, err)
		}

		checkpointValue := ""
		if pag != nil {
			checkpointValue = state.Checkpoint()
		}
		n, captured, err := c.sendRecords(res, records, sink, parent, checkpointValue, extractor, tracker)
		if err != nil {
			return totalRecords, pageCount, err
		}
		totalRecords += n
		pageCount++

		c.appendCaptures(res.Name, captured)
		c.observe.Report(filament.SourceProgress{
			Kind:     filament.SourceProgressPageFetched,
			Resource: progressResource,
			Records:  int64(n),
			Bytes:    int64(len(raw)),
		})

		if pag == nil {
			// Single-request resource: we're done.
			return totalRecords, pageCount, nil
		}

		// Decode body for paginators that consume response paths (cursor, next_url).
		// Root-array bodies decode as nil — ok, those pagers don't use body.
		var bodyMap map[string]any
		_ = json.Unmarshal(raw, &bodyMap)

		state, err = pag.Next(resp, bodyMap, len(records))
		if err != nil {
			return totalRecords, pageCount, fmt.Errorf("paginator next on %s: %w", res.Name, err)
		}

		if state.Done {
			return totalRecords, pageCount, nil
		}
	}
}

// fetchPage builds and sends one page request, applying pagination + watermark
// overrides. Retries 429/5xx internally with backoff.
func (c *Connector) fetchPage(
	ctx context.Context,
	res manifest.Resource,
	parent Capture,
	state pagination.State,
	pag pagination.Paginator,
	tracker *incremental.Tracker,
) (*http.Response, []byte, error) {
	build := func(ctx context.Context) (*http.Request, error) {
		scope := c.scopeFor(parent, state.Cursor, tracker)

		// Render once so body-merge sees the templated body (parent.*,
		// config.*, state.* placeholders already substituted).
		rendered, err := request.RenderResource(res, scope)
		if err != nil {
			return nil, fmt.Errorf("render resource %q: %w", res.Name, err)
		}

		req, err := c.builder.BuildRendered(ctx, rendered, scope)
		if err != nil {
			return nil, err
		}

		// Pagination: overrides may inject body fields for body-strategies.
		// nil pag means the resource declares no pagination — skip Apply.
		var pagOverrides map[string]any
		if pag != nil {
			pagOverrides, err = pag.Apply(req, state)
			if err != nil {
				return nil, fmt.Errorf("paginator apply: %w", err)
			}
		}

		// Incremental: same — overrides for body-strategy watermarks. The
		// tracker holds its request lower bound fixed across the page walk while
		// independently advancing the watermark that will be committed. Skip
		// this once a paginator replaces the URL wholesale (next_url, Link
		// header), because that URL is already the server's continuation of the
		// filtered query. Cursor/offset/page strategies rebuild the request from
		// the manifest and need the fixed lower bound applied on every page.
		var trackerOverrides map[string]any
		if tracker != nil && state.NextURL == "" {
			trackerOverrides, err = tracker.Apply(req)
			if err != nil {
				return nil, fmt.Errorf("incremental apply: %w", err)
			}
		}

		// Re-encode body merging into the RENDERED template (not the raw
		// res.Body.Template) so {{ parent.* }} / {{ config.* }} / {{ state.* }}
		// fields are preserved on body-inject pagination.
		if len(pagOverrides) > 0 || len(trackerOverrides) > 0 {
			merged := request.MergeOverrides(rendered.Body.Template, pagOverrides, trackerOverrides)
			body, ct, err := mustEncode(rendered.Body.Encoding, merged)
			if err != nil {
				return nil, err
			}
			req.Body = io.NopCloser(body)
			req.ContentLength = -1
			if ct != "" {
				req.Header.Set("Content-Type", ct)
			}
		}
		return req, nil
	}

	return c.doRequest(ctx, build, res.Name)
}

// mustEncode picks an encoder for the resolved body. Defaults to JSON.
func mustEncode(encoding string, body any) (io.Reader, string, error) {
	enc, err := request.EncoderFor(encoding, body)
	if err != nil {
		return nil, "", err
	}
	return enc.Encode(body)
}

// doRequest sends one HTTP request with retry on 429/5xx. The build closure
// is invoked per attempt so request bodies can be re-read.
func (c *Connector) doRequest(
	ctx context.Context,
	build func(context.Context) (*http.Request, error),
	resourceName string,
) (*http.Response, []byte, error) {
	var serverRetries int
	for attempt := range maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, nil, err
		}

		req, err := build(ctx)
		if err != nil {
			return nil, nil, err
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("http: %w", err)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
		if err != nil {
			return resp, nil, fmt.Errorf("read response body: %w", err)
		}

		c.limiter.Observe(resp)

		switch {
		case resp.StatusCode == 429:
			delay := retryAfterDuration(resp, attempt)
			c.observe.Report(filament.SourceProgress{
				Kind:       filament.SourceProgressRateLimited,
				Resource:   resourceName,
				RetryAfter: delay,
			})
			if attempt+1 >= maxRetries {
				err := fmt.Errorf("retries exhausted after %d attempts on %s", maxRetries, resourceName)
				c.reportRetryExhausted(resourceName, err)
				return nil, nil, err
			}
			if err := sleepCtx(ctx, delay); err != nil {
				return nil, nil, err
			}
		case resp.StatusCode >= 500 && serverRetries < maxServerErrRetries:
			serverRetries++
			c.logger.Warn("retrying server error",
				"resource", resourceName, "status", resp.StatusCode,
				"attempt", serverRetries, "max", maxServerErrRetries)
			if err := sleepCtx(ctx, retryAfterDuration(resp, serverRetries)); err != nil {
				return nil, nil, err
			}
		case resp.StatusCode >= 500:
			err := fmt.Errorf("%s %s HTTP %d: %s",
				resourceName, req.URL.Redacted(),
				resp.StatusCode, formatHTTPErrorBody(body))
			c.reportRetryExhausted(resourceName, err)
			return resp, body, err
		case resp.StatusCode >= 400:
			return resp, body, fmt.Errorf("%s %s HTTP %d: %s",
				resourceName, req.URL.Redacted(),
				resp.StatusCode, formatHTTPErrorBody(body))
		default:
			return resp, body, nil
		}
	}
	err := fmt.Errorf("retries exhausted after %d attempts on %s", maxRetries, resourceName)
	c.reportRetryExhausted(resourceName, err)
	return nil, nil, err
}

func (c *Connector) reportRetryExhausted(resource string, err error) {
	c.observe.Report(filament.SourceProgress{
		Kind:     filament.SourceProgressRetryExhausted,
		Resource: resource,
		Error:    err.Error(),
	})
}

func formatHTTPErrorBody(body []byte) string {
	var envelope struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Errors) > 0 && envelope.Errors[0].Message != "" {
		return errs.FormatTruncated(envelope.Errors[0].Message)
	}
	return errs.FormatTruncated(string(body))
}

// retryAfterDuration parses the Retry-After header (seconds or HTTP-date),
// otherwise computes exponential backoff capped at maxRetryBackoff.
func retryAfterDuration(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return min(time.Duration(secs)*time.Second, maxRetryBackoff)
		}
		if t, err := http.ParseTime(ra); err == nil {
			d := time.Until(t)
			if d <= 0 {
				return time.Second
			}
			return min(d, maxRetryBackoff)
		}
	}
	return min(time.Second<<uint(attempt), maxRetryBackoff)
}

// sleepCtx waits for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendRecords decodes each record into the sink, captures parent fields,
// and observes watermark fields.
func (c *Connector) sendRecords(
	res manifest.Resource,
	records []map[string]any,
	sink recordSink,
	parent Capture,
	cursor string,
	extractor *response.Extractor,
	tracker *incremental.Tracker,
) (int, []Capture, error) {
	resourceName, err := emittedResourceName(res, parent, c.env)
	if err != nil {
		return 0, nil, err
	}
	// Filter pushdown: drop records whose discovered ID is disabled. Applied
	// before marshal/emit/capture so disabled parents neither write records
	// nor fan out to children.
	if filter, ok := c.enabledByResource[res.Name]; ok {
		idPath := c.enabledIDPath[res.Name]
		filtered := records[:0]
		for _, rec := range records {
			id, _, _ := paths.AsString(rec, idPath)
			if _, allow := filter[id]; allow {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	var captured []Capture
	n := 0
	emitResource := !res.CaptureOnly && len(c.enabledResources) == 0
	if !emitResource && !res.CaptureOnly {
		_, emitResource = c.enabledResources[res.Name]
	}
	for _, rec := range records {
		if fields := extractor.Capture(rec, res.Capture); fields != nil {
			// A grandchild often needs both a value from this record and scope
			// inherited from its parent. Preserve that ancestry without making
			// manifests project synthetic parent.* paths from the raw response.
			for key, value := range parent {
				if _, exists := fields[key]; !exists {
					fields[key] = value
				}
			}
			captured = append(captured, fields)
		}
		if !emitResource {
			continue
		}
		data, projected, err := projectRecord(res, rec, parent)
		if err != nil {
			return n, captured, err
		}
		keyJSON, err := json.Marshal(extractKey(data, res.PrimaryKey))
		if err != nil {
			return n, captured, fmt.Errorf("marshal key: %w", err)
		}
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return n, captured, fmt.Errorf("marshal data: %w", err)
		}
		wr := newHTTPRecord(resourceName, keyJSON, dataJSON, projected)
		if cursor != "" {
			wr.Key = []string{cursor}
		}
		if tracker != nil {
			advanced, err := tracker.ObserveChecked(rec)
			if err != nil {
				return n, captured, fmt.Errorf("incremental cursor: %w", err)
			}
			if advanced {
				c.reportWatermarkOnce(resourceName, res.Incremental.DurableCheckpointKey(), tracker.Current())
			}
			if tracker.Current() != "" {
				wr.Key = watermarkKey(tracker.Current())
			}
		}
		if err := sink.Push(wr); err != nil {
			return n, captured, err
		}
		n++

	}
	return n, captured, nil
}

func extractKey(record map[string]any, primaryKey []string) map[string]any {
	if len(primaryKey) == 0 {
		return nil
	}
	key := make(map[string]any, len(primaryKey))
	for _, pk := range primaryKey {
		key[pk] = record[pk]
	}
	return key
}
