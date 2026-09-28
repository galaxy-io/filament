package connection

import (
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

// baseCfg builds a valid connection config with the given workspace URL.
func baseCfg(workspaceURL string) filament.Config {
	return filament.NewConfig(map[string]any{
		"workspace_url":    workspaceURL,
		"zerobus_endpoint": "wsid.zerobus.us-west-2.cloud.databricks.com",
		"client_id":        "cid",
		"client_secret":    "secret",
	})
}

// TestResolveRejectsNonHTTPSWorkspaceURL checks non-https and scheme-less URLs are rejected.
func TestResolveRejectsNonHTTPSWorkspaceURL(t *testing.T) {
	for _, bad := range []string{
		"http://ws.cloud.databricks.com",
		"ws.cloud.databricks.com", // scheme-less
		"ftp://ws.cloud.databricks.com",
		"https://", // no host
	} {
		if _, err := Resolve(baseCfg(bad)); err == nil {
			t.Errorf("workspace_url %q: expected error, got nil", bad)
		} else if !strings.Contains(err.Error(), "https") {
			t.Errorf("workspace_url %q: error = %v, want https message", bad, err)
		}
	}
}

// TestResolveAcceptsHTTPSWorkspaceURL checks a valid https workspace URL resolves.
func TestResolveAcceptsHTTPSWorkspaceURL(t *testing.T) {
	r, err := Resolve(baseCfg("https://ws.cloud.databricks.com"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.WorkspaceURL != "https://ws.cloud.databricks.com" {
		t.Fatalf("WorkspaceURL = %q", r.WorkspaceURL)
	}
}
