package connection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testSecret is distinctive so a leak into an error message is detectable.
const testSecret = "s3cr3t-value-123"

// workspace fakes the token endpoint and the APIs Test calls. A status of 0
// keeps the default success response.
type workspace struct {
	tokenStatus, meStatus, catalogStatus, warehouseStatus int
	calls                                                 []string
}

func (w *workspace) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.calls = append(w.calls, r.URL.Path)
		switch {
		case r.URL.Path == "/oidc/v1/token":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
			}
			if r.PostForm.Get("client_id") != "cid" || r.PostForm.Get("client_secret") != testSecret {
				t.Errorf("token request credentials = %q/%q", r.PostForm.Get("client_id"), r.PostForm.Get("client_secret"))
			}
			if w.tokenStatus != 0 {
				rw.Header().Set("Content-Type", "application/json")
				rw.WriteHeader(w.tokenStatus)
				_, _ = rw.Write([]byte(`{"error":"invalid_client","error_description":"Client authentication failed"}`))
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			_, _ = rw.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
		case r.Header.Get("Authorization") != "Bearer tok":
			t.Errorf("%s: Authorization = %q", r.URL.Path, r.Header.Get("Authorization"))
			rw.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/api/2.0/preview/scim/v2/Me":
			respond(rw, w.meStatus, `{"error_code":"PERMISSION_DENIED","message":"no access to workspace"}`)
		case r.URL.Path == "/api/2.1/unity-catalog/catalogs/main":
			respond(rw, w.catalogStatus, `{"error_code":"CATALOG_DOES_NOT_EXIST","message":"catalog not found"}`)
		case r.URL.Path == "/api/2.0/sql/warehouses/wh1":
			respond(rw, w.warehouseStatus, `{"error_code":"RESOURCE_DOES_NOT_EXIST","message":"warehouse not found"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			rw.WriteHeader(http.StatusNotFound)
		}
	})
}

func respond(rw http.ResponseWriter, status int, errBody string) {
	rw.Header().Set("Content-Type", "application/json")
	if status != 0 {
		rw.WriteHeader(status)
		_, _ = rw.Write([]byte(errBody))
		return
	}
	_, _ = rw.Write([]byte(`{}`))
}

// serve starts a TLS fake workspace and points the package transport at it.
func serve(t *testing.T, w *workspace) Resolved {
	t.Helper()
	srv := httptest.NewTLSServer(w.handler(t))
	t.Cleanup(srv.Close)
	prev := transport
	transport = srv.Client().Transport
	t.Cleanup(func() { transport = prev })
	return Resolved{WorkspaceURL: srv.URL + "/", ClientID: "cid", ClientSecret: testSecret, Catalog: "main"}
}

// TestTestChecksCredentialsAndWorkspace checks a valid principal passes after
// one token exchange, one authenticated workspace call and a catalog lookup.
func TestTestChecksCredentialsAndWorkspace(t *testing.T) {
	w := &workspace{}
	if err := Test(context.Background(), serve(t, w)); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if got := strings.Join(w.calls, ","); got != "/oidc/v1/token,/api/2.0/preview/scim/v2/Me,/api/2.1/unity-catalog/catalogs/main" {
		t.Fatalf("calls = %s", got)
	}
}

// TestTestChecksWarehouseWhenSet checks the warehouse is probed only when configured.
func TestTestChecksWarehouseWhenSet(t *testing.T) {
	w := &workspace{}
	r := serve(t, w)
	r.WarehouseID = "wh1"
	if err := Test(context.Background(), r); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if got := w.calls[len(w.calls)-1]; got != "/api/2.0/sql/warehouses/wh1" {
		t.Fatalf("last call = %s, want warehouse lookup", got)
	}
}

// TestTestReportsFailures checks each failing step returns an error naming it.
func TestTestReportsFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    workspace
		want []string
	}{
		{"bad credentials", workspace{tokenStatus: http.StatusUnauthorized}, []string{"authenticate service principal", "invalid_client"}},
		{"no workspace access", workspace{meStatus: http.StatusForbidden}, []string{"workspace", "HTTP 403", "no access to workspace"}},
		{"missing catalog", workspace{catalogStatus: http.StatusNotFound}, []string{`catalog "main"`, "HTTP 404", "catalog not found"}},
		{"missing warehouse", workspace{warehouseStatus: http.StatusNotFound}, []string{`warehouse "wh1"`, "HTTP 404", "warehouse not found"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := serve(t, &tc.w)
			r.WarehouseID = "wh1"
			err := Test(context.Background(), r)
			if err == nil {
				t.Fatal("Test: expected error, got nil")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			if strings.Contains(err.Error(), testSecret) {
				t.Errorf("error leaks the client secret: %q", err)
			}
		})
	}
}
