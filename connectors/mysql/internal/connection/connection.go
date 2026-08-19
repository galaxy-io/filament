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

const defaultPort = 3306

func Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("MySQL URL or driver DSN"),
		{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "MySQL server hostname"},
		{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "MySQL server port"},
		{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "MySQL username"},
		{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "MySQL password"},
		{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Database to connect to"},
		{Name: "tls_mode", Type: filament.FieldEnum, Default: "preferred", Enum: []filament.EnumOption{
			{Value: "false", Label: "Disable"},
			{Value: "preferred", Label: "Prefer"},
			{Value: "true", Label: "Require"},
			{Value: "skip-verify", Label: "Require, skip verification"},
		}, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "MySQL TLS mode"},
	}
}

func Resolve(cfg filament.Config) (*mysql.Config, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return nil, err
	}
	if method == dbconfig.MethodURL {
		dsn := cfg.Secret(dbconfig.DSNField)
		if strings.TrimSpace(dsn) == "" {
			return nil, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
		}
		parsed, err := parseDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse dsn: %w", err)
		}
		return parsed, nil
	}

	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return nil, fmt.Errorf("host is required when connection_method is %q", dbconfig.MethodFields)
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}
	username := cfg.String("username")
	if username == "" {
		return nil, fmt.Errorf("username is required when connection_method is %q", dbconfig.MethodFields)
	}
	password := cfg.Secret("password")
	if password == "" {
		return nil, fmt.Errorf("password is required when connection_method is %q", dbconfig.MethodFields)
	}
	database := cfg.String("database_name")
	if database == "" {
		return nil, fmt.Errorf("database_name is required when connection_method is %q", dbconfig.MethodFields)
	}
	tlsMode := cfg.String("tls_mode")
	if tlsMode == "" {
		tlsMode = "preferred"
	}
	switch tlsMode {
	case "false", "preferred", "true", "skip-verify":
	default:
		return nil, fmt.Errorf("unsupported tls_mode %q", tlsMode)
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	resolved := mysql.NewConfig()
	resolved.User = username
	resolved.Passwd = password
	resolved.Net = "tcp"
	resolved.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	resolved.DBName = database
	resolved.TLSConfig = tlsMode
	return mysql.ParseDSN(resolved.FormatDSN())
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
