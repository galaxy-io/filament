package auth

import (
	"context"
	"net/http"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("bearer", newBearer) }

type bearerAuth struct {
	token string // may be a template like "{{ config.api_key }}"
}

func newBearer(params map[string]any) (Authenticator, error) {
	tok, err := requireStrParam(params, "bearer", "token")
	if err != nil {
		return nil, err
	}
	return &bearerAuth{token: tok}, nil
}

func (a *bearerAuth) Apply(_ context.Context, req *http.Request, scope template.Scope) error {
	tok, err := template.Render(a.token, scope)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

// DeclaredWrites reports that bearer always writes the Authorization header.
func (a *bearerAuth) DeclaredWrites() []Write {
	return []Write{{Kind: WriteHeader, Name: "Authorization"}}
}
