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

// Resolved contains the PostgreSQL connection string and parsed pool config.
type Resolved struct {
	DSN          string
	DriverConfig *pgxpool.Config
}

// Resolve translates connector configuration into a PostgreSQL pool config.
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
	return Resolved{DSN: dsn, DriverConfig: pool}, nil
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
