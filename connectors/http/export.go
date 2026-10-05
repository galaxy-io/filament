package httpapi

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/internal/export"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// exportControl preserves the shared connector's authenticated request policy.
type exportControl struct{ connector *Connector }

func (a exportControl) Start(ctx context.Context, start manifest.Resource, scope template.Scope) ([]byte, error) {
	return a.connector.startExport(ctx, start, scope)
}

func (a exportControl) Poll(ctx context.Context, name string, req manifest.ExportRequest, scope template.Scope) ([]byte, error) {
	return a.connector.pollExport(ctx, name, req, scope)
}

func (c *Connector) exportRuntime(res manifest.Resource) *export.Runtime {
	config := map[string]string{}
	for key, value := range c.creds {
		if c.manifest.Config[key].Type != "secret" {
			config[key] = value
		}
	}
	runtime := &export.Runtime{
		WaitDownload: c.limiter.Wait,
		Control:      exportControl{connector: c}, Transport: c.streamClient.Transport,
		RetryAfter: retryAfterDuration, MaxRetries: maxRetries,
		BaseURL: c.builder.BaseURL, Config: c.creds, Env: c.env, IdentityConfig: config,
		ResumeToken: c.resumeExports[res.Name], Incremental: c.incrementalEnabled(res.Name, res.Name),
	}
	if c.builder.Auth != nil {
		runtime.Authenticate = c.builder.Auth.Apply
	}
	if value, ok := c.incrementalLookbacks[res.Name]; ok {
		runtime.Lookback = &value
	}
	return runtime
}

func (c *Connector) runExport(ctx context.Context, res manifest.Resource, sink recordSink, parents []Capture) error {
	checkpoints, ok := sink.(responseCheckpointSink)
	if !ok {
		return fmt.Errorf("export %s requires a checkpoint-capable sink", res.Name)
	}
	adapter := &exportSink{connector: c, resource: res, sink: sink, responseCheckpointSink: checkpoints}
	return c.exportRuntime(res).Run(ctx, res, adapter, parents)
}

// exportSink keeps projection, row metadata, and progress reporting in the
// connector. The underlying sink serializes concurrent row/checkpoint writes.
type exportSink struct {
	connector *Connector
	resource  manifest.Resource
	sink      recordSink
	responseCheckpointSink
}

func (s *exportSink) Emit(row map[string]any, parent map[string]string, token string) (int, error) {
	count, _, err := s.connector.sendRecords(s.resource, []map[string]any{row}, s.sink, parent, token, response.New(s.resource.Response), nil)
	return count, err
}

func (s *exportSink) Completed(count int) {
	s.connector.observe.Report(filament.SourceProgress{Kind: filament.SourceProgressPageFetched, Resource: s.resource.Name, Records: int64(count)})
}
