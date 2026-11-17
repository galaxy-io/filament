package alchemy

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	functionsv1 "github.com/galaxy-io/alchemy/api/functions/v1"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/transform"
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

func TestIdempotencyKeyCoversScopeAndInput(t *testing.T) {
	fn := transform.FunctionRef{ID: "fn", Version: 1}
	key := func(scope, input string) string {
		return idempotencyKey(Request{Function: fn, Scope: scope}, []byte(input))
	}
	if key("run_1", "a") != key("run_1", "a") {
		t.Fatal("an identical retry got a different key")
	}
	other := idempotencyKey(Request{Function: transform.FunctionRef{ID: "fn", Version: 2}, Scope: "run_1"}, []byte("a"))
	keys := map[string]bool{key("run_1", "a"): true, key("run_2", "a"): true, key("run_1", "b"): true, other: true}
	if len(keys) != 4 {
		t.Fatal("distinct scope, input or version shared a key")
	}
}
