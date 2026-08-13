// Package auth provisions filament's OIDC application in Zitadel at boot and
// exposes the resulting client configuration to the web UI.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Config is what the UI needs to run the PKCE flow. A nil *Config means auth
// is disabled. The PAT and verifier stay server-side for the proxies and the
// RPC interceptor.
type Config struct {
	Issuer   string `json:"issuer"`
	ClientID string `json:"clientId"`
	pat      string
	// projectID owns the filament roles; ownerOrgID is the org holding the
	// project. Other orgs need a project grant before user grants, cached
	// in projectGrants (org id -> grant id).
	projectID     string
	ownerOrgID    string
	projectGrants sync.Map
	verifier      *oidc.IDTokenVerifier
}

// FromEnv provisions the OIDC app when AUTH_ISSUER is set and returns its
// client config. AUTH_PAT_FILE is the Zitadel machine-user PAT minted at
// first instance; AUTH_UI_ORIGIN is the browser origin the login flow
// redirects back to.
func FromEnv(ctx context.Context) (*Config, error) {
	issuer := os.Getenv("AUTH_ISSUER")
	if issuer == "" {
		return nil, nil
	}
	patFile := os.Getenv("AUTH_PAT_FILE")
	if patFile == "" {
		return nil, fmt.Errorf("AUTH_ISSUER is set but AUTH_PAT_FILE is not")
	}
	pat, err := os.ReadFile(patFile) //nolint:gosec // operator-supplied path from env
	if err != nil {
		return nil, fmt.Errorf("read AUTH_PAT_FILE: %w", err)
	}
	origin := os.Getenv("AUTH_UI_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	project, clientID, err := ensureApp(ctx, strings.TrimRight(issuer, "/"), strings.TrimSpace(string(pat)), origin)
	if err != nil {
		return nil, fmt.Errorf("ensure oidc app: %w", err)
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover issuer %s: %w", issuer, err)
	}
	return &Config{
		Issuer:     issuer,
		ClientID:   clientID,
		pat:        strings.TrimSpace(string(pat)),
		projectID:  project.id,
		ownerOrgID: project.ownerOrgID,
		verifier:   provider.Verifier(&oidc.Config{SkipClientIDCheck: true}),
	}, nil
}

// Zitadel puts the user's organization on the token when the client requests
// the urn:zitadel:iam:user:resourceowner scope. The org id is the tenant id.
const (
	claimOrgID   = "urn:zitadel:iam:user:resourceowner:id"
	claimOrgName = "urn:zitadel:iam:user:resourceowner:name"
)

// Caller is the authenticated identity a bearer token resolves to.
type Caller struct {
	UserID  string
	OrgID   string
	OrgName string
	Roles   []string
}

// IsAdmin reports whether the caller holds the admin role.
func (c Caller) IsAdmin() bool {
	return slices.Contains(c.Roles, RoleAdmin)
}

// verifyBearer validates the Authorization header and returns the caller.
func (c *Config) verifyBearer(ctx context.Context, header http.Header) (Caller, error) {
	raw, ok := strings.CutPrefix(header.Get("Authorization"), "Bearer ")
	if !ok || raw == "" {
		return Caller{}, fmt.Errorf("missing bearer token")
	}
	token, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return Caller{}, err
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return Caller{}, err
	}
	orgID, _ := claims[claimOrgID].(string)
	if orgID == "" {
		return Caller{}, fmt.Errorf("token carries no organization; request the resourceowner scope")
	}
	orgName, _ := claims[claimOrgName].(string)
	sub, _ := claims["sub"].(string)
	caller := Caller{UserID: sub, OrgID: orgID, OrgName: orgName}
	if granted, ok := claims["urn:zitadel:iam:org:project:"+c.projectID+":roles"].(map[string]any); ok {
		for role := range granted {
			caller.Roles = append(caller.Roles, role)
		}
	}
	return caller, nil
}
