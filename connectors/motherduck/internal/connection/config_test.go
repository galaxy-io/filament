package connection

import (
	"net/url"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/version"
)

func TestResolveSetsUserAgent(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{"token": "secret", "database": "my_db"}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Path != "md:my_db" {
		t.Fatalf("path = %q, want %q", resolved.Path, "md:my_db")
	}
	if got, want := resolved.Options["custom_user_agent"], "filament/"+version.Version; got != want {
		t.Fatalf("custom_user_agent = %q, want %q", got, want)
	}
	if resolved.Options["motherduck_token"] != "secret" {
		t.Fatalf("motherduck_token = %q", resolved.Options["motherduck_token"])
	}
}

func TestDSNCarriesOptions(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{"token": "secret", "database": "my_db"}))
	if err != nil {
		t.Fatal(err)
	}
	dsn := resolved.DSN()
	if !strings.HasPrefix(dsn, "md:my_db?") {
		t.Fatalf("dsn = %q", dsn)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("motherduck_token") != "secret" {
		t.Fatalf("motherduck_token = %q", query.Get("motherduck_token"))
	}
	if query.Get("custom_user_agent") != UserAgent() {
		t.Fatalf("custom_user_agent = %q, want %q", query.Get("custom_user_agent"), UserAgent())
	}
}

func TestResolveRejectsMissingFields(t *testing.T) {
	if _, err := Resolve(filament.NewConfig(map[string]any{"database": "my_db"})); err == nil {
		t.Fatal("expected error for missing token")
	}
	if _, err := Resolve(filament.NewConfig(map[string]any{"token": "secret"})); err == nil {
		t.Fatal("expected error for missing database")
	}
}
