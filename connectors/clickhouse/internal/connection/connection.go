package connection

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

// Resolved contains the native ClickHouse driver configuration.
type Resolved struct {
	DriverConfig *ch.Options
}

// Resolve translates connector configuration into ClickHouse driver options.
func Resolve(cfg filament.Config) (Resolved, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return Resolved{}, err
	}
	if method == dbconfig.MethodURL {
		dsn := cfg.Secret(dbconfig.DSNField)
		if strings.TrimSpace(dsn) == "" {
			return Resolved{}, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
		}
		opts, err := ch.ParseDSN(dsn)
		if err != nil {
			return Resolved{}, fmt.Errorf("parse dsn: %w", err)
		}
		if len(opts.Addr) == 0 {
			return Resolved{}, fmt.Errorf("dsn has no server address")
		}
		return Resolved{DriverConfig: opts}, nil
	}

	host, err := normalizeHost(cfg.String("host"))
	if err != nil {
		return Resolved{}, err
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return Resolved{}, fmt.Errorf("port must be between 1 and 65535")
	}

	protocol := cfg.String("protocol")
	if protocol == "" {
		protocol = protocolNative
	}
	var driverProtocol ch.Protocol
	switch protocol {
	case protocolNative:
		driverProtocol = ch.Native
	case protocolHTTP:
		driverProtocol = ch.HTTP
	default:
		return Resolved{}, fmt.Errorf("unsupported protocol %q", protocol)
	}

	username := cfg.String("username")
	if username == "" {
		username = defaultUsername
	}
	password := cfg.Secret("password")
	if password == "" {
		return Resolved{}, fmt.Errorf("password is required (the configured secret may not have been resolved)")
	}
	opts := &ch.Options{
		Addr:     []string{net.JoinHostPort(host, strconv.Itoa(port))},
		Protocol: driverProtocol,
		Auth: ch.Auth{
			Database: defaultString(cfg.String("database_name"), DefaultDatabase),
			Username: username,
			Password: password,
		},
	}
	if driverProtocol == ch.Native {
		opts.Compression = &ch.Compression{Method: ch.CompressionLZ4}
	}
	secure := !cfg.Has("secure") || cfg.Bool("secure")
	if secure {
		opts.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return Resolved{DriverConfig: opts}, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func normalizeHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.Contains(host, "://") {
		endpoint, err := url.Parse(host)
		if err != nil {
			return "", fmt.Errorf("invalid host endpoint: %w", err)
		}
		if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
			return "", fmt.Errorf("host endpoint scheme must be http or https")
		}
		if endpoint.User != nil || (endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return "", fmt.Errorf("host endpoint must not contain credentials, a path, query parameters, or a fragment")
		}
		if endpoint.Port() != "" {
			return "", fmt.Errorf("host endpoint must not contain a port; use the port field")
		}
		host = endpoint.Hostname()
		if host == "" {
			return "", fmt.Errorf("host endpoint has no hostname")
		}
	}
	return strings.Trim(host, "[]"), nil
}
