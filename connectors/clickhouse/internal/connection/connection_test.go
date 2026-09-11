package connection

import (
	"strings"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/galaxy-io/filament"
)

func TestResolveFromDSN(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"connection_method": "url",
		"dsn":               "clickhouse://loader:secret@localhost:9000/analytics?secure=false",
	}))
	if err != nil {
		t.Fatal(err)
	}
	opts := resolved.DriverConfig
	if len(opts.Addr) != 1 || opts.Addr[0] != "localhost:9000" || opts.Auth.Username != "loader" || opts.Auth.Password != "secret" || opts.Auth.Database != "analytics" {
		t.Fatalf("options = %#v", opts)
	}
}

func TestResolveFromFields(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"host": "https://service.example.clickhouse.cloud", "port": 9440,
		"username": "loader", "password": "secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	opts := resolved.DriverConfig
	if got := opts.Addr; len(got) != 1 || got[0] != "service.example.clickhouse.cloud:9440" {
		t.Fatalf("address = %v", got)
	}
	if opts.Protocol.String() != protocolNative || opts.TLS == nil {
		t.Fatalf("protocol = %s, TLS = %#v", opts.Protocol, opts.TLS)
	}
	if opts.Compression == nil || opts.Compression.Method != ch.CompressionLZ4 {
		t.Fatalf("native compression = %#v, want LZ4", opts.Compression)
	}
	if opts.Auth.Database != DefaultDatabase || opts.Auth.Username != "loader" || opts.Auth.Password != "secret" {
		t.Fatalf("auth = %+v", opts.Auth)
	}
}

func TestResolveSupportsPlainHTTP(t *testing.T) {
	resolved, err := Resolve(filament.NewConfig(map[string]any{
		"host": "::1", "port": 8123, "protocol": protocolHTTP, "secure": false, "password": "local-secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	opts := resolved.DriverConfig
	if got := opts.Addr[0]; got != "[::1]:8123" {
		t.Fatalf("address = %q", got)
	}
	if opts.Protocol.String() != protocolHTTP || opts.TLS != nil {
		t.Fatalf("protocol = %s, TLS = %#v", opts.Protocol, opts.TLS)
	}
	if opts.Compression != nil {
		t.Fatalf("HTTP compression = %#v, want disabled", opts.Compression)
	}
	if opts.Auth.Username != defaultUsername {
		t.Fatalf("username = %q", opts.Auth.Username)
	}
}

func TestResolveRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		want string
	}{
		{name: "missing host", cfg: map[string]any{}, want: "host is required"},
		{name: "invalid port", cfg: map[string]any{"host": "localhost", "port": 70000}, want: "port must be"},
		{name: "invalid protocol", cfg: map[string]any{"host": "localhost", "protocol": "postgres"}, want: "unsupported protocol"},
		{name: "port in host", cfg: map[string]any{"host": "https://localhost:8443"}, want: "use the port field"},
		{name: "path in host", cfg: map[string]any{"host": "https://localhost/query"}, want: "must not contain"},
		{name: "missing password", cfg: map[string]any{"host": "localhost"}, want: "password is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(filament.NewConfig(tt.cfg))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
