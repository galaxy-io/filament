package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	build := fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html>app")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		"assets/index-abc.js":  {Data: []byte("console.log(1)")},
		"assets/index-abc.css": {Data: []byte("body{}")},
	}
	handler := spaHandler(build)

	tests := []struct {
		name         string
		path         string
		status       int
		body         string
		cacheControl string
	}{
		{name: "root serves index.html", path: "/", status: http.StatusOK, body: "<!doctype html>app", cacheControl: "no-cache"},
		{name: "existing file is served", path: "/favicon.svg", status: http.StatusOK, body: "<svg/>"},
		{name: "asset is cached forever", path: "/assets/index-abc.js", status: http.StatusOK, body: "console.log(1)", cacheControl: "public, max-age=31536000, immutable"},
		{name: "client route falls back to index.html", path: "/pipelines/abc/canvas", status: http.StatusOK, body: "<!doctype html>app", cacheControl: "no-cache"},
		{name: "missing asset is a 404", path: "/assets/index-old.js", status: http.StatusNotFound},
		{name: "assets directory falls back to index.html", path: "/assets", status: http.StatusOK, body: "<!doctype html>app", cacheControl: "no-cache"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.body != "" && rec.Body.String() != tt.body {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.body)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
		})
	}
}
