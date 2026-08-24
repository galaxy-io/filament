package connection

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResolveFields(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"host": "2001:db8::1", "username": "user@example.com", "password": "p@ss:/word",
		"database_name": "app/data", "ssl_mode": "verify-full",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Pool.ConnConfig.Host != "2001:db8::1" || resolved.Pool.ConnConfig.Port != 5432 {
		t.Fatalf("address = %s:%d", resolved.Pool.ConnConfig.Host, resolved.Pool.ConnConfig.Port)
	}
	if resolved.Pool.ConnConfig.User != "user@example.com" || resolved.Pool.ConnConfig.Password != "p@ss:/word" || resolved.Pool.ConnConfig.Database != "app/data" {
		t.Fatalf("connection config = %#v", resolved.Pool.ConnConfig)
	}
}

func TestResolveLegacyDSN(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"dsn": "postgres://legacy:secret@localhost:5433/app?sslmode=disable",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Pool.ConnConfig.Host != "localhost" || resolved.Pool.ConnConfig.Port != 5433 {
		t.Fatalf("address = %s:%d", resolved.Pool.ConnConfig.Host, resolved.Pool.ConnConfig.Port)
	}
}

func TestResolveSelectedModeDoesNotMixInputs(t *testing.T) {
	_, err := Resolve(filament.NewConfig(map[string]any{
		"connection_method": "fields",
		"dsn":               "postgres://legacy:secret@localhost/app",
	}))
	if err == nil {
		t.Fatal("fields mode accepted the inactive dsn")
	}
}
