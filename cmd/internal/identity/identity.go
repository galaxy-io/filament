// Package identity selects the authentication provider.
package identity

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/galaxy-io/filament/identity"
	"github.com/galaxy-io/filament/identity/keycloak"
	"github.com/galaxy-io/filament/identity/zitadel"
)

// FromEnv selects the provider per AUTH_PROVIDER. Unset leaves filament
// unauthenticated: a nil provider is the disabled state, not an error.
// Every provider takes AUTH_ISSUER and AUTH_UI_ORIGIN, the origin the UI is
// served from; an https origin marks the session cookie Secure.
//
// AUTH_BOOTSTRAP_TENANT is converged on every boot. AUTH_BOOTSTRAP_ADMIN_EMAIL
// and AUTH_BOOTSTRAP_ADMIN_PASSWORD administer it and close sign-up:
// registration is refused.
func FromEnv(ctx context.Context) (identity.Provider, error) {
	switch provider := os.Getenv("AUTH_PROVIDER"); provider {
	case "":
		return nil, nil
	// The machine-user token in AUTH_PAT.
	case "zitadel":
		pat := os.Getenv("AUTH_PAT")
		if pat == "" {
			return nil, errors.New("identity: AUTH_PAT is required for AUTH_PROVIDER=zitadel")
		}
		return zitadel.New(ctx, zitadel.Options{
			Issuer:   os.Getenv("AUTH_ISSUER"),
			PAT:      pat,
			UIOrigin: uiOrigin(),
			Bootstrap: zitadel.Bootstrap{
				Tenant:        os.Getenv("AUTH_BOOTSTRAP_TENANT"),
				AdminEmail:    os.Getenv("AUTH_BOOTSTRAP_ADMIN_EMAIL"),
				AdminPassword: os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
			},
		})
	// AUTH_ISSUER is the realm URL and filament's client is AUTH_CLIENT_ID and
	// AUTH_CLIENT_SECRET. With the admin in AUTH_ADMIN_USERNAME and
	// AUTH_ADMIN_PASSWORD it creates the realm and client itself. The
	// bootstrap tenant also takes an admin service account in
	// AUTH_BOOTSTRAP_CLIENT_ID and AUTH_BOOTSTRAP_CLIENT_SECRET.
	case "keycloak":
		clientID, clientSecret := os.Getenv("AUTH_CLIENT_ID"), os.Getenv("AUTH_CLIENT_SECRET")
		if clientID == "" || clientSecret == "" {
			return nil, errors.New("identity: AUTH_CLIENT_ID and AUTH_CLIENT_SECRET are required for AUTH_PROVIDER=keycloak")
		}
		return keycloak.New(ctx, keycloak.Options{
			Issuer:        os.Getenv("AUTH_ISSUER"),
			ClientID:      clientID,
			ClientSecret:  clientSecret,
			UIOrigin:      uiOrigin(),
			AdminUsername: os.Getenv("AUTH_ADMIN_USERNAME"),
			AdminPassword: os.Getenv("AUTH_ADMIN_PASSWORD"),
			Bootstrap: keycloak.Bootstrap{
				Tenant:        os.Getenv("AUTH_BOOTSTRAP_TENANT"),
				ClientID:      os.Getenv("AUTH_BOOTSTRAP_CLIENT_ID"),
				ClientSecret:  os.Getenv("AUTH_BOOTSTRAP_CLIENT_SECRET"),
				AdminEmail:    os.Getenv("AUTH_BOOTSTRAP_ADMIN_EMAIL"),
				AdminPassword: os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
			},
		})
	default:
		return nil, fmt.Errorf("identity: unknown AUTH_PROVIDER %q (zitadel, keycloak)", provider)
	}
}

func uiOrigin() string {
	if origin := os.Getenv("AUTH_UI_ORIGIN"); origin != "" {
		return origin
	}
	return "http://localhost:5173"
}
