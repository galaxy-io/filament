// Package identity selects the authentication provider.
package identity

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/galaxy-io/filament/identity"
	"github.com/galaxy-io/filament/identity/zitadel"
)

// FromEnv selects the provider per AUTH_PROVIDER. Unset leaves filament
// unauthenticated: a nil provider is the disabled state, not an error.
//
// zitadel needs AUTH_ISSUER and the machine-user token in AUTH_PAT;
// AUTH_UI_ORIGIN is the browser origin sign-in returns to.
func FromEnv(ctx context.Context) (identity.Provider, error) {
	switch provider := os.Getenv("AUTH_PROVIDER"); provider {
	case "":
		return nil, nil
	case "zitadel":
		pat := os.Getenv("AUTH_PAT")
		if pat == "" {
			return nil, errors.New("identity: AUTH_PAT is required for AUTH_PROVIDER=zitadel")
		}
		return zitadel.New(ctx, zitadel.Options{
			Issuer:   os.Getenv("AUTH_ISSUER"),
			PAT:      pat,
			UIOrigin: uiOrigin(),
		})
	default:
		return nil, fmt.Errorf("identity: unknown AUTH_PROVIDER %q (zitadel)", provider)
	}
}

func uiOrigin() string {
	if origin := os.Getenv("AUTH_UI_ORIGIN"); origin != "" {
		return origin
	}
	return "http://localhost:5173"
}
