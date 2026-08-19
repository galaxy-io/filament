package connection

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const defaultPort = 5432

type Resolved struct {
	DSN  string
	Pool *pgxpool.Config
}

func Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("PostgreSQL connection URL"),
		{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "PostgreSQL server hostname"},
		{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "PostgreSQL server port"},
		{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "PostgreSQL username"},
		{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "PostgreSQL password"},
		{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Database to connect to"},
		{Name: "ssl_mode", Type: filament.FieldEnum, Default: "prefer", Enum: []filament.EnumOption{
			{Value: "disable", Label: "Disable"},
			{Value: "allow", Label: "Allow"},
			{Value: "prefer", Label: "Prefer"},
			{Value: "require", Label: "Require"},
			{Value: "verify-ca", Label: "Verify CA"},
			{Value: "verify-full", Label: "Verify Full"},
		}, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "PostgreSQL TLS verification mode"},
	}
}

func Resolve(cfg filament.Config) (Resolved, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return Resolved{}, err
	}
	dsn := cfg.Secret(dbconfig.DSNField)
	if method == dbconfig.MethodFields {
		dsn, err = fieldsDSN(cfg)
		if err != nil {
			return Resolved{}, err
		}
	} else if strings.TrimSpace(dsn) == "" {
		return Resolved{}, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
	}
	pool, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return Resolved{}, fmt.Errorf("parse dsn: %w", err)
	}
	return Resolved{DSN: dsn, Pool: pool}, nil
}

func fieldsDSN(cfg filament.Config) (string, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return "", fmt.Errorf("host is required when connection_method is %q", dbconfig.MethodFields)
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	username := cfg.String("username")
	if username == "" {
		return "", fmt.Errorf("username is required when connection_method is %q", dbconfig.MethodFields)
	}
	password := cfg.Secret("password")
	if password == "" {
		return "", fmt.Errorf("password is required when connection_method is %q", dbconfig.MethodFields)
	}
	database := cfg.String("database_name")
	if database == "" {
		return "", fmt.Errorf("database_name is required when connection_method is %q", dbconfig.MethodFields)
	}
	sslMode := cfg.String("ssl_mode")
	if sslMode == "" {
		sslMode = "prefer"
	}
	switch sslMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", fmt.Errorf("unsupported ssl_mode %q", sslMode)
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	u := &url.URL{
		Scheme:  "postgresql",
		User:    url.UserPassword(username, password),
		Host:    net.JoinHostPort(host, strconv.Itoa(port)),
		Path:    "/" + database,
		RawPath: "/" + url.PathEscape(database),
	}
	query := u.Query()
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()
	return u.String(), nil
}
