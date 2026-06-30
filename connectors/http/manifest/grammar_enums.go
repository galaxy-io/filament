package manifest

import (
	"fmt"
	"slices"
)

// Enum values for the v2 manifest. These are the authoritative lists for
// validateSemantics. grammar.v2.json mirrors them for IDE autocomplete and
// pre-decode shape checks; keep both in sync when adding a value.
//
// Empty string is permitted in many enums to represent "use the default"
// (Normalize fills in the actual default before validation runs).
var (
	ValidHTTPMethods   = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", ""}
	ValidModes         = []string{"paginated", "stream", ""}
	ValidBodyEncodings = []string{"json", "form", "multipart", "raw", "none", ""}
	ValidResponseRoots = []string{"array", "object", ""}

	ValidPaginationTypes  = []string{"cursor", "offset", "page", "link_header", "next_url", "none", ""}
	ValidPaginationInject = []string{"body", "query", "header", ""}

	ValidIncrementalInject = []string{"query", "body", "header"}
	ValidComparators       = []string{"lex", "numeric", "time", ""}

	ValidStreamTypes = []string{"ndjson", "sse", "chunked_array"}

	ValidRateLimitResetFormats = []string{"unix_seconds", "seconds_from_now", "http_date", ""}
)

// checkEnum errors when v is not in allowed. Caller wraps the returned error
// with a manifest path so the author sees both the bad value and where it
// landed in their YAML.
func checkEnum(v string, allowed []string) error {
	if slices.Contains(allowed, v) {
		return nil
	}
	return fmt.Errorf("%q is not one of %v", v, allowed)
}
