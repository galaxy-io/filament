package ui

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"
)

func TestProductURLsFromEnv(t *testing.T) {
	env := map[string]string{
		"GALAXY_ALCHEMY_URL":  "https://alchemy.example.com/",
		"GALAXY_FILAMENT_URL": "https://galaxy.example.com/ingest",
		"GALAXY_OTHER_URL":    "https://other.example.com",
	}
	got, err := ProductURLsFromEnv(func(name string) string { return env[name] })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"alchemy": "https://alchemy.example.com", "filament": "https://galaxy.example.com/ingest"}
	if !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, err := ProductURLsFromEnv(func(string) string { return "" }); err != nil || len(got) != 0 {
		t.Errorf("unset: got %v, %v, want no products", got, err)
	}
}

func TestProductURLsFromEnvRejects(t *testing.T) {
	for _, input := range []string{
		"alchemy.example.com",
		"/functions",
		"ftp://alchemy.example.com",
		"javascript:alert(1)",
		"https://alchemy.example.com?x=1",
		"https://alchemy.example.com#x",
		"https://user@alchemy.example.com",
		"https://",
	} {
		t.Run(input, func(t *testing.T) {
			got, err := ProductURLsFromEnv(func(name string) string {
				if name == "GALAXY_ALCHEMY_URL" {
					return input
				}
				return ""
			})
			if err == nil {
				t.Errorf("got %v, want an error", got)
			} else if !strings.HasPrefix(err.Error(), "GALAXY_ALCHEMY_URL: ") {
				t.Errorf("error %q does not name the variable", err)
			}
		})
	}
}

func TestRenderIndex(t *testing.T) {
	got, err := renderIndex([]byte("<head></head>"), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if want := `<head><script type="application/json" id="filament-runtime-config">{}</script></head>`; string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
	hostile := Config{ProductURLs: map[string]string{"alchemy": "https://a.example.com/</script><script>alert(1)"}}
	got, err = renderIndex([]byte("<head></head>"), hostile)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), "</script>"); n != 1 {
		t.Fatalf("got %d </script> tags, want 1: %s", n, got)
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(string(got), `<head><script type="application/json" id="filament-runtime-config">`), "</script></head>")
	var decoded Config
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(decoded.ProductURLs, hostile.ProductURLs) {
		t.Errorf("decoded %v, want %v", decoded.ProductURLs, hostile.ProductURLs)
	}
	if _, err := renderIndex([]byte("<html></html>"), Config{}); err == nil {
		t.Error("want an error for an index.html without </head>")
	}
}
