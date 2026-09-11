package connection

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

// Resolved contains the native MySQL driver configuration.
type Resolved struct {
	DriverConfig *mysql.Config
}

// Resolve translates connector configuration into a MySQL driver config.
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
		parsed, err := parseDSN(dsn)
		if err != nil {
			return Resolved{}, fmt.Errorf("parse dsn: %w", err)
		}
		return Resolved{DriverConfig: parsed}, nil
	}

	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return Resolved{}, fmt.Errorf("host is required when connection_method is %q", dbconfig.MethodFields)
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return Resolved{}, fmt.Errorf("port must be between 1 and 65535")
	}
	username := cfg.String("username")
	if username == "" {
		return Resolved{}, fmt.Errorf("username is required when connection_method is %q", dbconfig.MethodFields)
	}
	password := cfg.Secret("password")
	if password == "" {
		return Resolved{}, fmt.Errorf("password is required when connection_method is %q", dbconfig.MethodFields)
	}
	database := cfg.String("database_name")
	if database == "" {
		return Resolved{}, fmt.Errorf("database_name is required when connection_method is %q", dbconfig.MethodFields)
	}
	tlsMode := cfg.String("tls_mode")
	if tlsMode == "" {
		tlsMode = "preferred"
	}
	switch tlsMode {
	case "false", "preferred", "true", "skip-verify":
	default:
		return Resolved{}, fmt.Errorf("unsupported tls_mode %q", tlsMode)
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	resolved := mysql.NewConfig()
	resolved.User = username
	resolved.Passwd = password
	resolved.Net = "tcp"
	resolved.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	resolved.DBName = database
	resolved.TLSConfig = tlsMode
	driverConfig, err := mysql.ParseDSN(resolved.FormatDSN())
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{DriverConfig: driverConfig}, nil
}

// parseDSN accepts both the go-sql-driver native DSN and the conventional
// mysql:// URL form. The latter is translated to a driver Config before being
// normalized by ParseDSN so both inputs have identical defaults and TLS state.
func parseDSN(raw string) (*mysql.Config, error) {
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "mysql://") || strings.HasPrefix(lower, "mariadb://") {
		parsedURL, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse mysql URL: %w", err)
		}
		if parsedURL.Hostname() == "" {
			return nil, fmt.Errorf("mysql URL has no hostname")
		}
		if parsedURL.Fragment != "" {
			return nil, fmt.Errorf("mysql URL must not contain a fragment")
		}
		port := parsedURL.Port()
		if port == "" {
			port = strconv.Itoa(defaultPort)
		}
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portNumber == 0 {
			return nil, fmt.Errorf("mysql URL port must be between 1 and 65535")
		}

		cfg := mysql.NewConfig()
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(parsedURL.Hostname(), port)
		if parsedURL.User != nil {
			cfg.User = parsedURL.User.Username()
			cfg.Passwd, _ = parsedURL.User.Password()
		}
		escapedDatabase := strings.TrimPrefix(parsedURL.EscapedPath(), "/")
		cfg.DBName, err = url.PathUnescape(escapedDatabase)
		if err != nil {
			return nil, fmt.Errorf("invalid database name: %w", err)
		}
		driverDSN := cfg.FormatDSN()
		if parsedURL.RawQuery != "" {
			driverDSN += "?" + parsedURL.RawQuery
		}
		return mysql.ParseDSN(driverDSN)
	}
	return mysql.ParseDSN(raw)
}
