package connection

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResolveFields(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"host": "2001:db8::1", "username": "loader", "password": "p@ss:/word",
		"database_name": "app", "tls_mode": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Addr != "[2001:db8::1]:3306" || resolved.User != "loader" || resolved.Passwd != "p@ss:/word" || resolved.DBName != "app" || resolved.TLSConfig != "true" {
		t.Fatalf("connection config = %#v", resolved)
	}
}

func TestResolveLegacyDSN(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"dsn": "legacy:secret@tcp(localhost:3307)/app?tls=false",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Addr != "localhost:3307" || resolved.User != "legacy" || resolved.DBName != "app" {
		t.Fatalf("connection config = %#v", resolved)
	}
}

func TestResolveRejectsInvalidPort(t *testing.T) {
	_, err := Resolve(filament.NewConfig(map[string]any{
		"host": "localhost", "port": 70000, "username": "loader", "password": "secret", "database_name": "app",
	}))
	if err == nil {
		t.Fatal("invalid port accepted")
	}
}
