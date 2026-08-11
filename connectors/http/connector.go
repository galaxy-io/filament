// Package httpapi is the generic Tier-2 HTTP connector. It is driven by a v1
// YAML manifest and composes a pluggable auth, request builder, paginator,
// response extractor, watermark tracker, and stream reader to extract records
// from arbitrary REST/SSE/streaming APIs.
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/galaxy-io/filament/connectors/http/auth"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline/integrity"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
	"github.com/galaxy-io/filament/connectors/http/request"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Capture is one parent record's flattened, captured field map keyed by the
// `capture:` block name from the manifest. Captured per-record during a
// parent's extraction, then consumed as the `parent.*` template scope of
// child resource requests during fan-out.
//
// Values are always strings — Capture is intentionally flat. See
// response.Extractor.Capture for extraction semantics (non-scalar leaves
// silently become "").
type Capture = map[string]string

const (
	defaultChildConcurrency = 5
	defaultClientTimeout    = 60 * time.Second
	maxResponseSize         = 100 * 1024 * 1024 // 100MB
	maxRetries              = 10
	maxServerErrRetries     = 3
	maxRetryBackoff         = 60 * time.Second
)

// Connector is the generic HTTP connector. Configure with a v1 manifest and
// per-source credentials, then call Extract.
type Connector struct {
	manifestPath string
	manifestData []byte
	creds        map[string]string

	manifest *manifest.Manifest
	builder  *request.Builder
	limiter  request.Limiter

	client       *http.Client
	streamClient *http.Client // no timeout — caller-controlled via ctx

	logger   *slog.Logger
	reporter pipeline.Reporter

	// commitCheckpoint, when non-nil, is the runner-supplied write target for
	// connector state that needs to survive across runs (incremental
	// watermarks + captures rehydration). Set per-extract from opts.Checkpoint;
	// nil means the extraction is non-resumable and connector state lives only
	// in memory.
	commitCheckpoint *integrity.PipelineCheckpoint

	// parentRecords collects each parent record's captured fields so that
	// child resources can fan out across them. Keyed by parent resource name;
	// each value is the slice of per-record captures emitted by the parent's
	// `capture:` block during extraction.
	mu            sync.Mutex
	parentRecords map[string][]Capture

	// enabledByResource, keyed by manifest resource name, holds the set of
	// connector-side IDs the caller wants extracted. Built per-extract from
	// ExtractOptions.EnabledResources cross-referenced against
	// manifest.Discovery (which resource produces which kind). nil for a
	// resource means "no filter" — legacy extract-everything behavior. Read
	// only after Connector.extract has set it.
	enabledByResource map[string]map[string]struct{}
	enabledIDPath     map[string]string
	enabledResources  map[string]struct{}
	resumeCursors     map[string]string
	resumeWatermarks  map[string]map[string]string

	// watermarkReported tracks which resources have already emitted a
	// EventWatermarkAdvanced event during this extraction. The event is
	// fired once per resource (on first advance) rather than per record
	// to keep the event stream sparse — operators just need to know "this
	// resource is making incremental progress", not the value at every
	// row.
	watermarkReported sync.Map // resource name -> struct{}
}

// SetManifestPath is called by the registry before Configure.
func (c *Connector) SetManifestPath(path string) { c.manifestPath = path }

// SetManifestData configures the connector from embedded manifest bytes.
func (c *Connector) SetManifestData(data []byte) { c.manifestData = data }

// SetCredentials injects the `config.*` template scope. Built per-source by
// the registry's CredentialExtractor; the httpapi package stays proto-agnostic.
func (c *Connector) SetCredentials(m map[string]string) { c.creds = m }

// Spec reports the connector's name, tier, and supported modes.
func (c *Connector) Spec() pipeline.ConnectorSpec {
	name := "http"
	if c.manifest != nil {
		name = c.manifest.Name
	}
	return pipeline.ConnectorSpec{
		Name:  name,
		Tier:  pipeline.TierHTTP,
		Modes: []pipeline.ConnectorMode{pipeline.ModeBatch, pipeline.ModeStream},
	}
}

// Validate checks that a manifest path or embedded manifest data is set.
func (c *Connector) Validate() error {
	if c.manifestPath == "" && len(c.manifestData) == 0 {
		return fmt.Errorf("manifest_path is required")
	}
	return nil
}

// Configure loads the manifest and builds the auth, rate limiter, and HTTP clients.
func (c *Connector) Configure(ctx context.Context) error {
	if c.manifestPath == "" && len(c.manifestData) == 0 {
		return fmt.Errorf("manifest_path not set — call SetManifestPath before Configure")
	}

	var (
		m   *manifest.Manifest
		err error
	)
	if len(c.manifestData) > 0 {
		m, err = manifest.Parse(c.manifestData)
		if err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
	} else {
		m, err = manifest.Load(c.manifestPath)
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}
	}
	c.manifest = m

	authn, err := buildAuth(m.Connection.Auth, c.creds)
	if err != nil {
		return fmt.Errorf("build auth: %w", err)
	}

	c.builder = &request.Builder{
		BaseURL:           m.Connection.BaseURL,
		ConnectionHeaders: m.Connection.Headers,
		Auth:              authn,
	}
	c.limiter = buildLimiter(m.Connection.RateLimit)

	timeout := defaultClientTimeout
	if m.Connection.TimeoutSeconds > 0 {
		timeout = time.Duration(m.Connection.TimeoutSeconds) * time.Second
	}
	c.client = &http.Client{Timeout: timeout}
	c.streamClient = &http.Client{}
	c.parentRecords = make(map[string][]Capture)

	// Default observability so any pre-extract code path (Validate, Configure,
	// Teardown) that logs or reports does so against safe defaults rather than
	// nil-checking everywhere. Replaced per-extract by extract().
	c.logger = obs.Logger(c.logger)
	c.reporter = obs.Reporter(c.reporter)

	// ctx is unused: manifest loading is local file IO and auth.Build does no
	// network work. Kept in the signature for forward compatibility — when
	// auth strategies grow startup probes (oauth2 token preflight, etc.) they
	// will need ctx to honour caller deadlines.
	_ = ctx
	return nil
}

// TestConnection performs one authenticated request against the first
// top-level, non-streaming resource in the manifest. It deliberately stops
// after the first response: validation should prove that the credentials are
// accepted without walking pagination or extracting user data.
func (c *Connector) TestConnection(ctx context.Context) error {
	if c.manifest == nil || c.builder == nil || c.client == nil {
		return fmt.Errorf("connector is not configured")
	}
	var probe *manifest.Resource
	for i := range c.manifest.Resources {
		candidate := &c.manifest.Resources[i]
		if candidate.Parent == nil && candidate.Mode != "stream" {
			probe = candidate
			break
		}
	}
	if probe == nil {
		return fmt.Errorf("manifest has no top-level request suitable for connection validation")
	}

	build := func(ctx context.Context) (*http.Request, error) {
		return c.builder.Build(ctx, *probe, template.Scope{Config: c.creds})
	}
	_, body, err := c.doRequest(ctx, build, probe.Name)
	if err != nil {
		return fmt.Errorf("connection probe: %w", err)
	}
	if err := response.New(probe.Response).CheckError(body); err != nil {
		return fmt.Errorf("connection probe: %w", err)
	}
	return nil
}

// Teardown closes idle connections on the connector's HTTP clients.
func (c *Connector) Teardown(_ context.Context) error {
	if c.client != nil {
		c.client.CloseIdleConnections()
	}
	if c.streamClient != nil {
		c.streamClient.CloseIdleConnections()
	}
	return nil
}

// buildAuth constructs an Authenticator from the manifest's AuthSpec. The
// AuthSpec.Params map is passed through verbatim; auth implementations
// validate their own required keys. Static credential fields can use the
// `{{ config.* }}` template (resolved per-request inside Apply).
//
// creds threads the connector's `config.*` scope into auth.Build so missing
// config keys referenced by auth params surface as Configure errors, not
// first-request 401s.
func buildAuth(spec manifest.AuthSpec, creds map[string]string) (auth.Authenticator, error) {
	return auth.Build(spec.Type, spec.Params, creds)
}

// buildLimiter constructs a Limiter from the manifest's RateLimit. Static
// when Dynamic is nil, dynamic-from-headers otherwise.
func buildLimiter(rl manifest.RateLimit) request.Limiter {
	if rl.Dynamic == nil {
		return request.NewStaticLimiter(rl.RequestsPerSecond)
	}
	return request.NewDynamicLimiter(rl.RequestsPerSecond, request.DynamicConfig{
		RemainingHeader: rl.Dynamic.RemainingHeader,
		ResetHeader:     rl.Dynamic.ResetHeader,
		ResetFormat:     rl.Dynamic.ResetFormat,
		MinFloorRPS:     rl.Dynamic.MinFloorRPS,
	})
}
