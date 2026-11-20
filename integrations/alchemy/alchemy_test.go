package alchemy

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	functionsv1 "github.com/galaxy-io/alchemy/api/functions/v1"

	"github.com/galaxy-io/filament"
)

type directory map[filament.TenantID]string

func (d directory) TenantExternalID(_ context.Context, id filament.TenantID) (string, error) {
	ext, ok := d[id]
	if !ok {
		return "", filament.ErrNotFound
	}
	return ext, nil
}

func TestCredentialsScopeEveryCall(t *testing.T) {
	auth := &authenticator{creds: ProxyCredentials{Service: "filament"}, tenants: directory{"t1": "org-1", "local": ""}}
	var reached connect.AnyRequest
	call := auth.intercept(func(_ context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		reached = req
		return nil, nil
	})

	req := connect.NewRequest(&functionsv1.GetFunctionRequest{})
	if _, err := call(WithTenant(context.Background(), "t1"), req); err != nil {
		t.Fatal(err)
	}
	if h := reached.Header(); h.Get("X-Forwarded-Tenant") != "org-1" || h.Get("X-Forwarded-Service") != "filament" {
		t.Fatalf("headers = %v", h)
	}

	for name, ctx := range map[string]context.Context{
		"no tenant":                 context.Background(),
		"tenant without a provider": WithTenant(context.Background(), "local"),
		"unknown tenant":            WithTenant(context.Background(), "ghost"),
	} {
		reached = nil
		if _, err := call(ctx, connect.NewRequest(&functionsv1.GetFunctionRequest{})); err == nil || reached != nil {
			t.Fatalf("%s: err = %v, reached alchemy = %v", name, err, reached != nil)
		}
	}
}
