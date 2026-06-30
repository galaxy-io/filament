// Package errs holds the httpapi connector's error taxonomy. Sub-packages
// import this rather than the top-level httpapi package to avoid an import
// cycle (httpapi imports its sub-packages, sub-packages can't import httpapi).
//
// Wrap sentinels with fmt.Errorf("...: %w", errs.ErrFoo) so callers can use
// errors.Is to dispatch on failure mode.
//
// # Worked example
//
// A path extractor distinguishes "field absent" from "field is null":
//
//	v, _, err := paths.AsString(record, "user.name")
//	switch {
//	case errors.Is(err, errs.ErrPathMissing):
//	    // user.name not present — capture as ""
//	case errors.Is(err, errs.ErrPathNull):
//	    // present but null — strategy may treat as terminator
//	case err != nil:
//	    return err  // wrong type, surface to caller
//	}
//
// # Failure modes
//
//   - ErrPathMissing / ErrPathNull / ErrPathType — path lookup outcomes
//   - ErrTemplateMissingKey / ErrTemplateSyntax — template Render failures
//   - ErrManifestValidate — wraps a *ManifestErrors aggregate; unwrap with
//     errors.As to walk every issue, or use Error() for a multi-line summary
//   - ErrTruncatedStream / ErrMalformedLine — stream reader failures
//   - ErrAuthRefresh — OAuth/token refresh failed; existing creds are stale
//   - ErrRateLimitParse — could not interpret rate-limit response headers
//
// FormatTruncated caps long upstream-body fragments before they're embedded
// in error messages — use it instead of inlining `string(body)` directly.
package errs

import (
	"errors"
	"fmt"
)

// Sentinel errors. Use errors.Is to test for these in callers and tests.
var (
	// ErrPathMissing means a dot-path lookup found no matching key in the
	// document tree (the key was absent, not present-with-null).
	ErrPathMissing = errors.New("httpapi: path not found")

	// ErrPathNull means a dot-path resolved to JSON null. Distinct from
	// ErrPathMissing so strategies can choose to treat null as terminator
	// vs. error.
	ErrPathNull = errors.New("httpapi: path resolved to null")

	// ErrPathType means a dot-path resolved to a value of an unexpected type
	// (e.g. asked for a string, got a number; asked for an array index on a
	// non-array).
	ErrPathType = errors.New("httpapi: path resolved to wrong type")

	// ErrTemplateMissingKey means a template referenced a scope key that
	// wasn't supplied and had no default.
	ErrTemplateMissingKey = errors.New("httpapi: template references missing key")

	// ErrTemplateSyntax means a template string failed to parse (bad braces,
	// unclosed default, unknown filter, etc).
	ErrTemplateSyntax = errors.New("httpapi: template syntax error")

	// ErrManifestValidate means manifest validation found one or more
	// problems. Callers should unwrap to a *ManifestErrors aggregate to see
	// every issue.
	ErrManifestValidate = errors.New("httpapi: manifest validation failed")

	// ErrTruncatedStream means a streaming reader hit EOF before the
	// expected document terminator (e.g. closing `]` for a JSON array).
	ErrTruncatedStream = errors.New("httpapi: stream truncated")

	// ErrMalformedLine means a line-delimited stream (NDJSON) contained a
	// line that failed to parse. Strategies may choose to count or fail.
	ErrMalformedLine = errors.New("httpapi: malformed stream line")

	// ErrAuthRefresh means an auth refresh attempt (OAuth token, etc.)
	// failed and the existing credential can't be used.
	ErrAuthRefresh = errors.New("httpapi: auth refresh failed")

	// ErrRateLimitParse means a rate-limit response header could not be
	// interpreted (bad number, unrecognized reset format).
	ErrRateLimitParse = errors.New("httpapi: rate-limit header parse failed")
)

// MaxErrorBodyBytes caps how many bytes of an upstream response body or other
// large string are included verbatim in wrapped errors. Used by FormatTruncated.
const MaxErrorBodyBytes = 500

// FormatTruncated returns s capped at MaxErrorBodyBytes with a trailing "..."
// when truncation occurred. Use when including upstream response bodies or
// other unbounded strings inside error messages.
func FormatTruncated(s string) string {
	return FormatTruncatedN(s, MaxErrorBodyBytes)
}

// FormatTruncatedN is FormatTruncated with a caller-supplied cap.
func FormatTruncatedN(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ManifestErrors aggregates multiple validation problems into one error so
// `httpapi-lint` and manifest load report every issue at once instead of one
// per run.
type ManifestErrors struct {
	Issues []ManifestIssue
}

// ManifestIssue locates one validation problem inside a manifest.
type ManifestIssue struct {
	Path    string // dotted location, e.g. "resources[2].pagination.cursor_path"
	Message string
}

// Error implements error. Lists every issue on its own line.
func (m *ManifestErrors) Error() string {
	if m == nil || len(m.Issues) == 0 {
		return "httpapi: manifest validation failed (no issues)"
	}
	if len(m.Issues) == 1 {
		return fmt.Sprintf("httpapi: manifest invalid: %s: %s", m.Issues[0].Path, m.Issues[0].Message)
	}
	out := fmt.Sprintf("httpapi: manifest invalid (%d issues):", len(m.Issues))
	for _, iss := range m.Issues {
		out += "\n  - " + iss.Path + ": " + iss.Message
	}
	return out
}

// Unwrap lets callers test `errors.Is(err, errs.ErrManifestValidate)`.
func (m *ManifestErrors) Unwrap() error { return ErrManifestValidate }

// Addf records one issue with printf-formatted message. Callers use Addf
// exclusively — there's no plain Add since every production site has a
// formatted message.
func (m *ManifestErrors) Addf(path, format string, args ...any) *ManifestErrors {
	m.Issues = append(m.Issues, ManifestIssue{Path: path, Message: fmt.Sprintf(format, args...)})
	return m
}

// AsError returns m if any issue is recorded, else nil. Use at the end of a
// validation pass: `return v.AsError()`.
func (m *ManifestErrors) AsError() error {
	if m == nil || len(m.Issues) == 0 {
		return nil
	}
	return m
}
