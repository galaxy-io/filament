package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/request"
)

func TestSecureRedirectRejectsDowngrade(t *testing.T) {
	httpsPrev := []*http.Request{{URL: &url.URL{Scheme: "https", Host: "store.example.com"}}}

	// HTTPS -> HTTP downgrade (where Go would otherwise forward Authorization
	// on the same host) must be refused.
	downgrade := &http.Request{URL: &url.URL{Scheme: "http", Host: "store.example.com"}}
	if err := secureRedirect(downgrade, httpsPrev); err == nil {
		t.Fatal("expected HTTPS->HTTP downgrade redirect to be refused, got nil")
	}

	// HTTPS -> HTTPS is fine.
	same := &http.Request{URL: &url.URL{Scheme: "https", Host: "cdn.store.example.com"}}
	if err := secureRedirect(same, httpsPrev); err != nil {
		t.Fatalf("expected HTTPS->HTTPS redirect to be allowed, got %v", err)
	}

	// HTTP -> HTTP is fine (local/dev httptest clients).
	httpPrev := []*http.Request{{URL: &url.URL{Scheme: "http", Host: "127.0.0.1:8080"}}}
	plain := &http.Request{URL: &url.URL{Scheme: "http", Host: "127.0.0.1:8080"}}
	if err := secureRedirect(plain, httpPrev); err != nil {
		t.Fatalf("expected HTTP->HTTP redirect to be allowed, got %v", err)
	}
}

func TestSourceValidatesEnumConfiguration(t *testing.T) {
	data := strings.Replace(responseTokenManifest, "tenant_id: {type: string}", `environment: {type: enum, enum: ["", integration], default: ""}`, 1)
	src := NewManifest([]byte(data))
	if src.manifestErr != nil {
		t.Fatal(src.manifestErr)
	}
	for _, tc := range []struct {
		config map[string]any
		valid  bool
	}{
		{nil, true},
		{map[string]any{"environment": ""}, true},
		{map[string]any{"environment": "integration"}, true},
		{map[string]any{"environment": "other"}, false},
	} {
		err := src.Validate(filament.NewConfig(tc.config))
		if (err == nil) != tc.valid {
			t.Fatalf("config=%v error=%v", tc.config, err)
		}
	}
}

type responseSizeTransport struct {
	size   int64
	closed bool
}

func (r *responseSizeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: &responseSizeBody{remaining: r.size, closed: &r.closed}, Request: req}, nil
}

type responseSizeBody struct {
	remaining int64
	closed    *bool
}

func (b *responseSizeBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), b.remaining)
	clear(p[:n])
	b.remaining -= n
	return int(n), nil
}
func (b *responseSizeBody) Close() error { *b.closed = true; return nil }

func TestResponseSizeBoundary(t *testing.T) {
	for _, extra := range []int64{0, 1} {
		transport := &responseSizeTransport{size: maxResponseSize + extra}
		c := &Connector{client: &http.Client{Transport: transport}, limiter: request.NewStaticLimiter(0), manifest: &manifest.Manifest{}}
		_, body, err := c.doRequest(t.Context(), func(ctx context.Context) (*http.Request, error) {
			return http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)
		}, manifest.Resource{Name: "items"})
		if !transport.closed {
			t.Fatal("response body not closed")
		}
		if extra == 0 {
			if err != nil || len(body) != maxResponseSize {
				t.Fatalf("limit-sized response: bytes=%d error=%v", len(body), err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "response exceeds") || len(body) != 0 {
			t.Fatalf("oversized response: bytes=%d error=%v", len(body), err)
		}
	}
}
