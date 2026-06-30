package auth

import (
	"context"
	"net/http"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("header", newHeader) }

// headerAuth sets an arbitrary HTTP header. Optional `scheme` is prepended
// (e.g. "Token", "ApiKey") with a single space separator.
type headerAuth struct {
	name   string
	value  string
	scheme string
}

func newHeader(params map[string]any) (Authenticator, error) {
	name, err := requireStrParam(params, "header", "name")
	if err != nil {
		return nil, err
	}
	value, err := requireStrParam(params, "header", "value")
	if err != nil {
		return nil, err
	}
	return &headerAuth{
		name:   name,
		value:  value,
		scheme: strParam(params, "scheme"),
	}, nil
}

func (a *headerAuth) Apply(_ context.Context, req *http.Request, scope template.Scope) error {
	val, err := template.Render(a.value, scope)
	if err != nil {
		return err
	}
	if a.scheme != "" {
		val = a.scheme + " " + val
	}
	req.Header.Set(a.name, val)
	return nil
}

// DeclaredWrites reports the configured header name.
func (a *headerAuth) DeclaredWrites() []Write {
	return []Write{{Kind: WriteHeader, Name: a.name}}
}
