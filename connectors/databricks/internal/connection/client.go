package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// testTimeout bounds each request TestConnection makes.
const testTimeout = 30 * time.Second

// transport is the base HTTP transport for workspace calls. Tests swap in the
// TLS test server's transport.
var transport http.RoundTripper = http.DefaultTransport

// oauthConfig is the OAuth machine-to-machine client-credentials flow for the
// service principal against the workspace's token endpoint.
func oauthConfig(r Resolved) *clientcredentials.Config {
	return &clientcredentials.Config{
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		TokenURL:     strings.TrimRight(r.WorkspaceURL, "/") + "/oidc/v1/token",
		Scopes:       []string{"all-apis"},
		AuthStyle:    oauth2.AuthStyleInParams,
	}
}

// HTTPClient returns an HTTP client that authenticates workspace API calls as
// the service principal, fetching and refreshing tokens as needed.
func HTTPClient(r Resolved, timeout time.Duration) *http.Client {
	base := &http.Client{Transport: transport, Timeout: timeout}
	client := oauthConfig(r).Client(context.WithValue(context.Background(), oauth2.HTTPClient, base))
	client.Timeout = timeout
	return client
}

// Test checks the credentials against the workspace. It exchanges the service
// principal's client ID and secret for a token, makes one authenticated call,
// and checks the principal can see the catalog, so a wrong workspace URL,
// client ID, secret or catalog fails here rather than on the first run. When
// warehouse_id is set it also checks the principal can see that warehouse,
// which create_table needs. Zerobus authorizes each stream for one table, so
// ingest permissions are only verified when a stream opens.
func Test(ctx context.Context, r Resolved) error {
	base := &http.Client{Transport: transport, Timeout: testTimeout}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, base)
	cfg := oauthConfig(r)
	token, err := cfg.Token(ctx)
	if err != nil {
		return fmt.Errorf("authenticate service principal: %w", err)
	}
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	client.Timeout = testTimeout
	workspace := strings.TrimRight(r.WorkspaceURL, "/")
	if err := get(ctx, client, workspace+"/api/2.0/preview/scim/v2/Me"); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}
	if err := get(ctx, client, workspace+"/api/2.1/unity-catalog/catalogs/"+url.PathEscape(r.Catalog)); err != nil {
		return fmt.Errorf("catalog %q: %w", r.Catalog, err)
	}
	if r.WarehouseID != "" {
		if err := get(ctx, client, workspace+"/api/2.0/sql/warehouses/"+url.PathEscape(r.WarehouseID)); err != nil {
			return fmt.Errorf("warehouse %q: %w", r.WarehouseID, err)
		}
	}
	return nil
}

// get issues an authenticated GET and treats any non-2xx status as an error,
// surfacing the Databricks error message when the body carries one.
func get(ctx context.Context, client *http.Client, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	var body struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
	if body.Message != "" {
		return fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, body.ErrorCode, body.Message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
