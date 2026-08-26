package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/connectors/http/stream"
)

// streamResource opens a single long-lived HTTP connection and emits records
// as they arrive. Format is selected via res.Stream (defaults to NDJSON for
// back-compat with mode: stream resources that omit a format).
//
//nolint:funlen // connection lifecycle and read loop belong together
func (c *Connector) streamResource(
	ctx context.Context,
	res manifest.Resource,
	sink recordSink,
	parent Capture,
	extractor *response.Extractor,
	tracker *incremental.Tracker,
) (int, error) {
	reader, err := stream.NewWithOptions(res.Stream, stream.Options{Logger: c.logger})
	if err != nil {
		return 0, fmt.Errorf("stream reader: %w", err)
	}

	scope := c.scopeFor(parent, "", tracker)
	req, err := c.builder.Build(ctx, res, scope)
	if err != nil {
		return 0, err
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", defaultStreamAccept(res.Stream))
	}

	resp, err := c.streamClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("stream request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		return 0, fmt.Errorf("stream %s/%s %s HTTP %d: %s",
			c.manifest.Name, res.Name, req.URL.Redacted(),
			resp.StatusCode, errs.FormatTruncated(string(body)))
	}

	resourceName, err := emittedResourceName(res, parent)
	if err != nil {
		return 0, err
	}
	var totalRecords int
	var captured []Capture

	var filter map[string]struct{}
	var filterIDPath string
	if f, ok := c.enabledByResource[res.Name]; ok {
		filter = f
		filterIDPath = c.enabledIDPath[res.Name]
	}

	emit := func(record map[string]any) error {
		if filter != nil {
			id, _, _ := paths.AsString(record, filterIDPath)
			if _, allow := filter[id]; !allow {
				return nil
			}
		}
		data, projected, err := projectRecord(res, record, parent)
		if err != nil {
			return err
		}
		keyJSON, err := json.Marshal(extractKey(data, res.PrimaryKey))
		if err != nil {
			return fmt.Errorf("marshal stream key: %w", err)
		}
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal stream data: %w", err)
		}
		wr := newHTTPRecord(resourceName, keyJSON, dataJSON, projected)
		if tracker != nil {
			advanced, err := tracker.ObserveChecked(record)
			if err != nil {
				return fmt.Errorf("incremental cursor: %w", err)
			}
			if advanced {
				c.reportWatermarkOnce(resourceName, res.Incremental.DurableCheckpointKey(), tracker.Current())
			}
			if tracker.Current() != "" {
				wr.Key = watermarkKey(tracker.Current())
			}
		}
		if err := sink.Push(wr); err != nil {
			return fmt.Errorf("stream send: %w", err)
		}
		totalRecords++

		if fields := extractor.Capture(record, res.Capture); fields != nil {
			for key, value := range parent {
				if _, exists := fields[key]; !exists {
					fields[key] = value
				}
			}
			captured = append(captured, fields)
		}
		if totalRecords%1000 == 0 {
			c.observe.Report(filament.SourceProgress{
				Kind:     filament.SourceProgressPageFetched,
				Resource: resourceName,
				Records:  1000,
			})
		}

		return nil
	}

	if err := reader.Read(ctx, resp.Body, emit); err != nil {
		return totalRecords, fmt.Errorf("stream read: %w", err)
	}

	c.appendCaptures(res.Name, captured)

	return totalRecords, nil
}

func defaultStreamAccept(spec *manifest.StreamSpec) string {
	if spec == nil {
		return "application/x-ndjson"
	}
	switch spec.Type {
	case "sse":
		return "text/event-stream"
	case "chunked_array":
		return "application/json"
	}
	return "application/x-ndjson"
}
