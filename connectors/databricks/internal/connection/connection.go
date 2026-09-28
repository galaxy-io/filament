package connection

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/galaxy-io/filament"
)

// Resolved holds the validated Databricks connection settings. Catalog, schema
// and the pipeline-scoped destination options are read by the sink from the
// merged run config, not here.
type Resolved struct {
	WorkspaceURL    string
	ZerobusEndpoint string
	ClientID        string
	ClientSecret    string
	WarehouseID     string
}

// Resolve reads and validates the connection-scoped credentials and endpoints.
// It performs no network access.
func Resolve(cfg filament.Config) (Resolved, error) {
	r := Resolved{
		WorkspaceURL:    strings.TrimSpace(cfg.String("workspace_url")),
		ZerobusEndpoint: strings.TrimSpace(cfg.String("zerobus_endpoint")),
		ClientID:        strings.TrimSpace(cfg.String("client_id")),
		ClientSecret:    cfg.Secret("client_secret"),
		WarehouseID:     strings.TrimSpace(cfg.String("warehouse_id")),
	}
	if r.WorkspaceURL == "" {
		return Resolved{}, fmt.Errorf("workspace_url is required")
	}
	// The workspace URL carries the service-principal secret to the OAuth token
	// endpoint (and to the Zerobus SDK), so it must be an absolute https URL; a
	// plain-http or scheme-less value would transmit the secret in cleartext.
	if u, err := url.Parse(r.WorkspaceURL); err != nil || u.Scheme != "https" || u.Host == "" {
		return Resolved{}, fmt.Errorf("workspace_url must be an absolute https URL")
	}
	if r.ZerobusEndpoint == "" {
		return Resolved{}, fmt.Errorf("zerobus_endpoint is required")
	}
	if r.ClientID == "" {
		return Resolved{}, fmt.Errorf("client_id is required")
	}
	if strings.TrimSpace(r.ClientSecret) == "" {
		return Resolved{}, fmt.Errorf("client_secret is required")
	}
	return r, nil
}
