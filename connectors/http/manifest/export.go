package manifest

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

// ExportSpec describes a finite asynchronous read. Request retries and pending
// job polls have separate budgets. Each parent has independent job state.
type ExportSpec struct {
	// Direct downloads a ready artifact without creating an upstream job.
	// It is limited to top-level full snapshots, which replay on interruption.
	Direct    bool         `yaml:"direct,omitempty"`
	ParentKey []string     `yaml:"parent_key,omitempty"`
	Start     ExportStart  `yaml:"start"`
	Wait      ExportWait   `yaml:"wait"`
	Result    ExportResult `yaml:"result"`
	Next      *ExportNext  `yaml:"next,omitempty"`
}

// ExportNext starts another job when a completed artifact is only one page.
// It is restricted to top-level full reads; next_cursor is reserved job state.
type ExportNext struct {
	MorePath   string        `yaml:"more_path"`
	CursorPath string        `yaml:"cursor_path"`
	Request    ExportRequest `yaml:"request"`
}

// ExportRequest is an authenticated, API-relative control request.
type ExportRequest struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Query   map[string]string `yaml:"query,omitempty"`
	Body    BodySpec          `yaml:"body,omitempty"`
}

// Resource adapts a control request to the shared HTTP request builder.
func (r ExportRequest) Resource(name string) Resource {
	return Resource{Name: name, Method: r.Method, Path: r.Path, Headers: r.Headers, Query: r.Query, Body: r.Body}
}

// ExportStart creates a job and captures its identity from the response.
type ExportStart struct {
	ExportRequest `yaml:",inline"`
	Capture       map[string]string `yaml:"capture"`
}

// ExportWait defines readiness polling and its finite time budget.
type ExportWait struct {
	Type            string            `yaml:"type"` // job | download
	Request         *ExportRequest    `yaml:"request,omitempty"`
	State           *ExportJobState   `yaml:"state,omitempty"`
	Capture         map[string]string `yaml:"capture,omitempty"`
	PendingStatuses []int             `yaml:"pending_statuses,omitempty"`
	IntervalSeconds int               `yaml:"interval_seconds"`
	TimeoutSeconds  int               `yaml:"timeout_seconds"`
}

// ExportJobState partitions job statuses into disjoint terminal and pending states.
type ExportJobState struct {
	Path    string   `yaml:"path"`
	Pending []string `yaml:"pending"`
	Ready   []string `yaml:"ready"`
	Failed  []string `yaml:"failed,omitempty"`
}

// ExportResult selects and decodes the downloaded artifact.
type ExportResult struct {
	URL                  string          `yaml:"url"`
	Auth                 string          `yaml:"auth,omitempty"` // none | connection (same API origin only)
	AllowedHosts         []string        `yaml:"allowed_hosts"`
	Compression          string          `yaml:"compression,omitempty"`  // none | gzip
	Archive              string          `yaml:"archive,omitempty"`      // none | zip | tar
	Files                string          `yaml:"files,omitempty"`        // path.Match pattern over archive member names
	Format               string          `yaml:"format"`                 // json | ndjson | csv
	RecordsPath          string          `yaml:"records_path,omitempty"` // array in a JSON document
	Envelope             *ExportEnvelope `yaml:"envelope,omitempty"`
	MaxDownloadBytes     int64           `yaml:"max_download_bytes,omitempty"`
	MaxUncompressedBytes int64           `yaml:"max_uncompressed_bytes,omitempty"`
}

// ExportEnvelope unwraps operation results such as Mailchimp batch responses.
// Every operation must succeed; partial exports are never reported as complete.
type ExportEnvelope struct {
	StatusPath  string `yaml:"status_path"`
	Success     []int  `yaml:"success"`
	BodyPath    string `yaml:"body_path"`
	RecordsPath string `yaml:"records_path,omitempty"`
}

func validateExport(agg *errs.ManifestErrors, at string, r Resource) {
	if r.Mode != "export" {
		if r.Export != nil {
			_ = agg.Addf(at+".export", "requires mode: export")
		}
		return
	}
	e := r.Export
	if e == nil {
		_ = agg.Addf(at+".export", "is required for mode: export")
		return
	}
	at += ".export"
	validateExportResourceFields(agg, at, r)
	if e.Next != nil {
		if r.Parent != nil || r.Incremental != nil || e.Wait.Type != "job" {
			_ = agg.Addf(at+".next", "requires a top-level full export with job polling")
		}
		if e.Next.MorePath == "" || e.Next.CursorPath == "" {
			_ = agg.Addf(at+".next", "requires more_path and cursor_path")
		}
		validateExportRequest(agg, at+".next.request", e.Next.Request, true)
		for _, captures := range []map[string]string{e.Start.Capture, e.Wait.Capture} {
			for _, key := range []string{"next_cursor", "next_more"} {
				if _, ok := captures[key]; ok {
					_ = agg.Addf(at+".capture", "%s is reserved for export continuation", key)
				}
			}
		}
	}
	validateExportParent(agg, at, r)
	if e.Direct {
		if e.Start.Path != "" || e.Start.Method != "" || len(e.Start.Capture) > 0 {
			_ = agg.Addf(at+".start", "cannot combine direct downloads with job creation")
		}
		if r.Parent != nil || r.Incremental != nil || e.Next != nil || e.Wait.Type != "download" {
			_ = agg.Addf(at+".direct", "requires a top-level full export with download waiting")
		}
		validateTemplateScopes(agg, at+".result.url", e.Result.URL, []string{"config", "env"})
	} else {
		validateExportRequest(agg, at+".start", e.Start.ExportRequest, false)
		if len(e.Start.Capture) == 0 {
			_ = agg.Addf(at+".start.capture", "must capture the job identity or download URL")
		}
	}
	validateExportWait(agg, at, e.Wait)
	for _, captures := range []map[string]string{e.Start.Capture, e.Wait.Capture} {
		for key, p := range captures {
			if key == "" || p == "" {
				_ = agg.Addf(at+".capture", "capture names and paths must be nonempty")
			}
		}
	}
	validateExportResult(agg, at, e.Result)
}

func validateExportWait(agg *errs.ManifestErrors, at string, wait ExportWait) {
	if wait.IntervalSeconds <= 0 || wait.IntervalSeconds > 86400 || wait.TimeoutSeconds <= 0 || wait.TimeoutSeconds > 604800 {
		_ = agg.Addf(at+".wait", "interval_seconds must be 1..86400 and timeout_seconds must be 1..604800")
	}
	switch wait.Type {
	case "job":
		if wait.Request == nil || wait.State == nil {
			_ = agg.Addf(at+".wait", "job polling requires request and state")
		} else {
			validateExportRequest(agg, at+".wait.request", *wait.Request, true)
			if wait.Request.Method != "GET" {
				_ = agg.Addf(at+".wait.request.method", "must be GET")
			}
			s := wait.State
			if s.Path == "" || len(s.Ready) == 0 || len(s.Pending) == 0 {
				_ = agg.Addf(at+".wait.state", "requires path, ready, and pending")
			}
			seen := map[string]bool{}
			for _, group := range [][]string{s.Ready, s.Pending, s.Failed} {
				for _, value := range group {
					if value == "" || seen[value] {
						_ = agg.Addf(at+".wait.state", "states must be nonempty and disjoint")
					}
					seen[value] = true
				}
			}
		}
		if len(wait.PendingStatuses) > 0 {
			_ = agg.Addf(at+".wait.pending_statuses", "only allowed for download polling")
		}
	case "download":
		if wait.Request != nil || wait.State != nil || len(wait.Capture) > 0 {
			_ = agg.Addf(at+".wait", "download polling cannot declare request, state, or capture")
		}
		for _, status := range wait.PendingStatuses {
			if !slices.Contains([]int{202, 403, 404, 409, 425}, status) {
				_ = agg.Addf(at+".wait.pending_statuses", "unsupported pending status %d", status)
			}
		}
	default:
		_ = agg.Addf(at+".wait.type", "must be job or download")
	}
}

func validateExportResult(agg *errs.ManifestErrors, at string, result ExportResult) {
	validateTemplateScopes(agg, at+".result.url", result.URL, []string{"config", "env", "parent", "state", "job"})
	if result.URL == "" || (result.Auth != "connection" && len(result.AllowedHosts) == 0) {
		_ = agg.Addf(at+".result", "requires url and allowed_hosts for unauthenticated downloads")
	}
	for _, host := range result.AllowedHosts {
		if host == "" || strings.ContainsAny(host, "/:@*?# ") {
			_ = agg.Addf(at+".result.allowed_hosts", "use exact hostnames without ports or wildcards")
		}
	}
	for field, check := range map[string]struct {
		value   string
		allowed []string
	}{
		"auth": {result.Auth, []string{"", "none", "connection"}}, "compression": {result.Compression, []string{"", "none", "gzip"}},
		"archive": {result.Archive, []string{"", "none", "zip", "tar"}}, "format": {result.Format, []string{"json", "ndjson", "csv"}},
	} {
		if err := checkEnum(check.value, check.allowed); err != nil {
			_ = agg.Addf(at+".result."+field, "%v", err)
		}
	}
	validateExportFiles(agg, at, result)
}

func validateExportFiles(agg *errs.ManifestErrors, at string, result ExportResult) {
	if result.Archive == "zip" && result.Compression == "gzip" {
		_ = agg.Addf(at+".result", "ZIP cannot be combined with gzip")
	}
	if result.Archive == "zip" || result.Archive == "tar" {
		if result.Files == "" {
			_ = agg.Addf(at+".result.files", "required for archives")
		}
		if _, err := path.Match(result.Files, ""); err != nil {
			_ = agg.Addf(at+".result.files", "invalid pattern")
		}
	} else if result.Files != "" {
		_ = agg.Addf(at+".result.files", "requires an archive")
	}
	if result.RecordsPath != "" && result.Format != "json" {
		_ = agg.Addf(at+".result.records_path", "requires JSON format")
	}
	if result.MaxDownloadBytes < 0 || result.MaxUncompressedBytes < 0 || result.MaxDownloadBytes > 1<<50 || result.MaxUncompressedBytes > 1<<50 {
		_ = agg.Addf(at+".result", "byte limits must be between 1 and 2^50 when set")
	}
	if envelope := result.Envelope; envelope != nil {
		if result.Format == "csv" || envelope.StatusPath == "" || envelope.BodyPath == "" || len(envelope.Success) == 0 {
			_ = agg.Addf(at+".result.envelope", "requires JSON/NDJSON, status_path, body_path, and success")
		}
		for _, status := range envelope.Success {
			if status < 200 || status >= 300 {
				_ = agg.Addf(at+".result.envelope.success", "must contain 2xx status codes")
			}
		}
	}
}

func validateExportRequest(agg *errs.ManifestErrors, at string, r ExportRequest, job bool) {
	scopes := []string{"config", "env", "parent", "state"}
	if job {
		scopes = append(scopes, "job")
	}
	if !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") {
		_ = agg.Addf(at+".path", "must be an API-relative path starting with /")
	}
	if r.Method != "POST" && r.Method != "GET" {
		_ = agg.Addf(at+".method", "must be GET or POST")
	}
	if err := checkEnum(r.Body.Encoding, ValidBodyEncodings); err != nil {
		_ = agg.Addf(at+".body.encoding", "%v", err)
	}
	validateTemplateScopes(agg, at+".path", r.Path, scopes)
	for k, v := range r.Headers {
		validateTemplateScopes(agg, at+".headers."+k, v, scopes)
	}
	for k, v := range r.Query {
		validateTemplateScopes(agg, at+".query."+k, v, scopes)
	}
	validateExportValue(agg, at+".body.template", r.Body.Template, scopes)
}

func validateExportValue(agg *errs.ManifestErrors, at string, value any, scopes []string) {
	switch v := value.(type) {
	case string:
		validateTemplateScopes(agg, at, v, scopes)
	case map[string]any:
		for k, item := range v {
			validateExportValue(agg, at+"."+k, item, scopes)
		}
	case []any:
		for i, item := range v {
			validateExportValue(agg, fmt.Sprintf("%s[%d]", at, i), item, scopes)
		}
	}
}

func validateExportParent(agg *errs.ManifestErrors, at string, r Resource) {
	if r.Parent != nil {
		if len(r.Export.ParentKey) == 0 {
			_ = agg.Addf(at+".parent_key", "required for parent-scoped exports")
		}
		if r.Parent.Since != "" {
			_ = agg.Addf(at, "parent.since is not supported; each export parent has its own watermark")
		}
	} else if len(r.Export.ParentKey) > 0 {
		_ = agg.Addf(at+".parent_key", "requires a parent resource")
	}
	if r.Incremental != nil && r.Incremental.ResponseCursor != "" {
		_ = agg.Addf(at, "exports use cursor_field, not response_cursor")
	}
}

func validateExportResourceFields(agg *errs.ManifestErrors, at string, r Resource) {
	if r.Stream != nil || r.EmitAs != "" || r.CaptureOnly || len(r.Capture) > 0 {
		_ = agg.Addf(at, "cannot combine export with stream, emit_as, capture, or capture_only")
	}
	if r.Path != "" || r.Method != "" || len(r.Headers) > 0 || len(r.Query) > 0 || len(r.Params) > 0 || r.Body.Encoding != "" || r.Body.Template != nil || r.Records != "" || (r.Pagination.Type != "" && r.Pagination.Type != "none") || r.Response != (ResponseSpec{}) {
		_ = agg.Addf(at, "declare requests and decoding inside export, without resource path, body, records, or pagination")
	}
}
