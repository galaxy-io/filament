package auth

import (
	"context"
	"net/http"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("query", newQuery) }

// queryAuth appends a query parameter to the request URL.
type queryAuth struct {
	param string
	value string
}

func newQuery(params map[string]any) (Authenticator, error) {
	param, err := requireStrParam(params, "query", "param")
	if err != nil {
		return nil, err
	}
	value, err := requireStrParam(params, "query", "value")
	if err != nil {
		return nil, err
	}
	return &queryAuth{param: param, value: value}, nil
}

func (a *queryAuth) Apply(_ context.Context, req *http.Request, scope template.Scope) error {
	val, err := template.Render(a.value, scope)
	if err != nil {
		return err
	}
	q := req.URL.Query()
	q.Set(a.param, val)
	req.URL.RawQuery = q.Encode()
	return nil
}

// DeclaredWrites reports the configured query parameter.
func (a *queryAuth) DeclaredWrites() []Write {
	return []Write{{Kind: WriteQueryParam, Name: a.param}}
}
