package sdk_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/identity"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
)

// Opt in with FILAMENT_SDK_PYTHON pointing to an interpreter with the SDK installed.
// The real Connect handlers and SQLite store exercise the generated wire format.
func TestPythonSDK(t *testing.T) {
	python := os.Getenv("FILAMENT_SDK_PYTHON")
	if python == "" {
		t.Skip("set FILAMENT_SDK_PYTHON to test the installed Python SDK")
	}
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	mux := http.NewServeMux()
	server.New(registry.NewSources(), registry.NewSinks(), store, nil, nil,
		server.WithIdentity(testIdentity{})).Mount(mux)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
			t.Errorf("Connect-Protocol-Version = %q, want 1", got)
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(api.Close)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "test_sdk.py", api.URL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Python SDK interoperability failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}

type testIdentity struct {
	authv1connect.UnimplementedAuthServiceHandler
}

func (testIdentity) Authenticate(_ context.Context, header http.Header) (identity.Caller, error) {
	if header.Get("Authorization") != "Bearer sdk-test" {
		return identity.Caller{}, errors.New("invalid test credential")
	}
	return identity.Caller{UserID: "sdk-user", TenantExternalID: "sdk-org"}, nil
}

func (testIdentity) GetAuthConfig(context.Context, *connect.Request[authv1.GetAuthConfigRequest]) (*connect.Response[authv1.GetAuthConfigResponse], error) {
	return connect.NewResponse(&authv1.GetAuthConfigResponse{Issuer: "https://identity.example.test"}), nil
}
