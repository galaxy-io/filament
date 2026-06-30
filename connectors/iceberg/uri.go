package iceberg

import (
	"path"
	"strings"
)

// joinURI joins a warehouse base (which may carry a scheme like "s3://bucket")
// with path segments without collapsing the scheme's "//". path.Join would turn
// "s3://bucket/p" into "s3:/bucket/p"; we split the scheme off, clean the rest,
// and reattach.
func joinURI(base string, segments ...string) string {
	scheme := ""
	rest := base
	if i := strings.Index(base, "://"); i >= 0 {
		scheme = base[:i+3] // include "://"
		rest = base[i+3:]
	}
	joined := path.Join(append([]string{rest}, segments...)...)
	return scheme + joined
}

// namespacePath turns a dot-separated namespace into a path fragment.
func namespacePath(namespace string) string {
	return strings.ReplaceAll(namespace, ".", "/")
}

// splitNamespace splits a dot-separated namespace into its parts.
func splitNamespace(namespace string) []string {
	return strings.Split(namespace, ".")
}
