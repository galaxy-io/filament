package compile

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestValidateOverlaySecretRefs(t *testing.T) {
	schema := filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "password", Type: filament.FieldSecret, Scope: filament.ScopeConnection},
		{Name: "api_key", Type: filament.FieldSecret}, // no scope: counts as a connection field
		{Name: "webhook_token", Type: filament.FieldSecret, Scope: filament.ScopePipeline},
		{Name: "topic", Type: filament.FieldString, Scope: filament.ScopePipeline},
		{Name: "auth", Type: filament.FieldObject, Scope: filament.ScopePipeline, Fields: []filament.ConfigField{
			{Name: "token", Type: filament.FieldSecret},
		}},
	}}

	cases := []struct {
		name    string
		refs    map[string]string
		wantErr bool
	}{
		{name: "no refs"},
		{name: "env ref on a pipeline field", refs: map[string]string{"webhook_token": "WEBHOOK_TOKEN"}},
		{name: "env ref inside a pipeline object", refs: map[string]string{"auth.token": "AUTH_TOKEN"}},

		{name: "ref for a connection field", refs: map[string]string{"password": "PG_PASSWORD"}, wantErr: true},
		{name: "ref for an unscoped field", refs: map[string]string{"api_key": "API_KEY"}, wantErr: true},
		{name: "ref for an unknown field", refs: map[string]string{"proxy_password": "PROXY"}, wantErr: true},
		{name: "ref for a non-secret pipeline field", refs: map[string]string{"topic": "TOPIC"}, wantErr: true},
		{name: "ref for an unknown nested field", refs: map[string]string{"auth.unknown": "AUTH"}, wantErr: true},
		{
			name:    "connection-managed ref on a pipeline field",
			refs:    map[string]string{"webhook_token": "filament/tenant/conn-1/password/v1/x"},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOverlaySecretRefs(schema, tc.refs)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateOverlaySecretRefs error = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}
