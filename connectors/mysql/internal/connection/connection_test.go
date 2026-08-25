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
	if resolved.TLS == nil || resolved.TLS.InsecureSkipVerify {
		t.Fatalf("normalized TLS config = %#v", resolved.TLS)
	}
}

func TestResolveMySQLURL(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"connection_method": "url",
		"dsn":               "mysql://user%40example.com:p%40ss%2Fword@[2001:db8::1]:3307/app%2Fdata?tls=true&parseTime=true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Addr != "[2001:db8::1]:3307" || resolved.User != "user@example.com" || resolved.Passwd != "p@ss/word" || resolved.DBName != "app/data" {
		t.Fatalf("connection config = %#v", resolved)
	}
	if resolved.TLS == nil || resolved.TLS.InsecureSkipVerify || !resolved.ParseTime {
		t.Fatalf("URL options were not normalized: TLS=%#v ParseTime=%v", resolved.TLS, resolved.ParseTime)
	}
}

func TestResolveMariaDBURLUsesDefaultPort(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"connection_method": "url",
		"dsn":               "mariadb://loader:secret@db.example.com/app?tls=preferred",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Addr != "db.example.com:3306" || resolved.TLS == nil || !resolved.AllowFallbackToPlaintext {
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

func TestResolveNativeDSNWithMySQLUsername(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"dsn": "mysql:secret@tcp(localhost:3307)/app",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.User != "mysql" || resolved.Passwd != "secret" {
		t.Fatalf("credentials = %q/%q", resolved.User, resolved.Passwd)
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

func TestResolveRejectsURLWithoutHost(t *testing.T) {
	_, err := Resolve(filament.NewConfig(map[string]any{
		"connection_method": "url",
		"dsn":               "mysql:///app",
	}))
	if err == nil {
		t.Fatal("URL without a host accepted")
	}
}
