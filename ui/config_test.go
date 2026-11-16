package ui

import (
	"strings"
	"testing"
)

func TestParseAlchemyURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "unset", input: "", want: ""},
		{name: "origin", input: "https://alchemy.example.com", want: "https://alchemy.example.com"},
		{name: "trailing slash", input: "https://alchemy.example.com/", want: "https://alchemy.example.com"},
		{name: "path kept", input: "https://galaxy.example.com/functions/", want: "https://galaxy.example.com/functions"},
		{name: "port kept", input: "http://localhost:8780", want: "http://localhost:8780"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAlchemyURL(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseAlchemyURLRejects(t *testing.T) {
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
			if got, err := ParseAlchemyURL(input); err == nil {
				t.Errorf("got %q, want an error", got)
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
	got, err = renderIndex([]byte("<head></head>"), Config{AlchemyURL: "https://a.example.com/</script>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "/</script></script>") {
		t.Errorf("config closed the script early: %s", got)
	}
	if _, err := renderIndex([]byte("<html></html>"), Config{}); err == nil {
		t.Error("want an error for an index.html without </head>")
	}
}
