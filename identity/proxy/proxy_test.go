package proxy

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
)

func TestAuthenticateUnpinned(t *testing.T) {
	caller, err := New("").Authenticate(context.Background(), http.Header{TenantHeader: {"tenant-2"}, UserHeader: {"u1"}})
	if err != nil {
		t.Fatal(err)
	}
	if caller.TenantExternalID != "tenant-2" || caller.UserID != "u1" {
		t.Fatalf("unexpected caller %+v", caller)
	}
}

func TestAuthenticate(t *testing.T) {
	a := New("tenant-1")
	for name, tc := range map[string]struct {
		header  http.Header
		code    connect.Code
		user    string
		service string
	}{
		"no tenant header": {
			header: http.Header{UserHeader: {"u1"}},
			code:   connect.CodeUnauthenticated,
		},
		"wrong tenant": {
			header: http.Header{TenantHeader: {"tenant-2"}, UserHeader: {"u1"}},
			code:   connect.CodePermissionDenied,
		},
		"no user header": {
			header: http.Header{TenantHeader: {"tenant-1"}},
			code:   connect.CodeUnauthenticated,
		},
		"user caller": {
			header: http.Header{TenantHeader: {"tenant-1"}, UserHeader: {"u1"}},
			user:   "u1",
		},
		"service caller": {
			header:  http.Header{TenantHeader: {"tenant-1"}, ServiceHeader: {"operator"}},
			service: "operator",
		},
		"service wins over user": {
			header:  http.Header{TenantHeader: {"tenant-1"}, UserHeader: {"u1"}, ServiceHeader: {"operator"}},
			service: "operator",
		},
	} {
		t.Run(name, func(t *testing.T) {
			caller, err := a.Authenticate(context.Background(), tc.header)
			if tc.code != 0 {
				if connect.CodeOf(err) != tc.code {
					t.Fatalf("got %v, want code %v", err, tc.code)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if caller.TenantExternalID != "tenant-1" || caller.UserID != tc.user || caller.Service != tc.service {
				t.Fatalf("unexpected caller %+v", caller)
			}
		})
	}
}
