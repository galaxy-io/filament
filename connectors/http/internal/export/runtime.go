// Package export runs finite asynchronous HTTP exports. It owns job lifecycles,
// artifact decoding, and recovery state; the connector supplies authenticated
// control requests and delivery into its normal projection and sink pipeline.
package export

import (
	"context"
	"net/http"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Control uses the connector's authentication, limiter, and request policy.
// Start accepts an already rendered request and must not retry creation.
// Poll renders the request against scope and may retry transient failures.
type Control interface {
	Start(context.Context, manifest.Resource, template.Scope) ([]byte, error)
	Poll(context.Context, string, manifest.ExportRequest, template.Scope) ([]byte, error)
}

// Sink serializes row delivery and checkpoint markers across concurrent jobs.
// Emit returns the number of delivered records. A nonempty token belongs on
// those rows; an empty token must not advance their checkpoint.
type Sink interface {
	Emit(row map[string]any, parent map[string]string, token string) (int, error)
	Checkpoint(resource, token string) error
	// Completed reports a validated artifact's records and response-body bytes
	// actually read, before manifest decompression or archive extraction.
	Completed(records int, downloadBytes int64)
}

// Runtime contains one extraction's immutable dependencies and resume options.
// IdentityConfig excludes secrets; Config and Env are used to render requests.
// Transport is used without API headers, auth, cookies, or client timeouts.
type Runtime struct {
	Observe filament.SourceObserver
	// WaitDownload applies the API limiter to authenticated artifact requests.
	WaitDownload func(context.Context) error
	// Locate reads a direct export's plain-text locator with the connector's
	// authentication and request policy, retrying transient failures.
	Locate                      func(context.Context, string, manifest.ExportRequest, template.Scope) ([]byte, error)
	Authenticate                func(context.Context, *http.Request, template.Scope) error
	Control                     Control
	Transport                   http.RoundTripper
	RetryAfter                  func(*http.Response, int) time.Duration
	MaxRetries                  int
	BaseURL                     string
	Config, Env, IdentityConfig map[string]string
	ResumeToken                 string
	Incremental                 bool
	Lookback                    *int
}

// Run creates or resumes the selected resource's jobs. Parents have already
// been enumerated by the connector; recovery may use the saved set instead.
func (c *Runtime) Run(ctx context.Context, res manifest.Resource, sink Sink, parents []map[string]string) error {
	if Scoped(res) {
		return c.runScoped(ctx, res, sink, parents)
	}
	return c.runFull(ctx, res, sink)
}

func (c *Runtime) scopeFor(parent map[string]string, tracker *incremental.Tracker) template.Scope {
	scope := template.Scope{Config: c.Config, Env: c.Env, Parent: parent}
	if tracker != nil {
		scope.State = tracker.Scope()
	}
	return scope
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
