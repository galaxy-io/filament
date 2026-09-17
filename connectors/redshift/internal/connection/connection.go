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

const applicationName = "filament"

// Resolved contains the validated database and load-staging configuration.
type Resolved struct {
	DriverConfig        *pgxpool.Config
	StagingBucket       string
	StagingBucketRegion string
	AssociatedIAMRole   string
}

// Resolve translates connector configuration into Redshift and S3 options.
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
		// pgx parse errors can echo connection-string fragments, including a
		// password. Keep this error intentionally generic.
		return Resolved{}, fmt.Errorf("parse dsn: invalid Redshift connection string")
	}
	if strings.TrimSpace(pool.ConnConfig.Host) == "" {
		return Resolved{}, fmt.Errorf("dsn must specify a host")
	}
	if strings.TrimSpace(pool.ConnConfig.User) == "" {
		return Resolved{}, fmt.Errorf("dsn must specify a username")
	}
	if strings.TrimSpace(pool.ConnConfig.Database) == "" {
		return Resolved{}, fmt.Errorf("dsn must specify a database")
	}
	if pool.ConnConfig.RuntimeParams == nil {
		pool.ConnConfig.RuntimeParams = make(map[string]string)
	}
	pool.ConnConfig.RuntimeParams["application_name"] = applicationName

	bucket := strings.TrimSpace(cfg.String("staging_bucket"))
	if bucket == "" {
		return Resolved{}, fmt.Errorf("staging_bucket is required")
	}
	if strings.Contains(bucket, "://") || strings.ContainsAny(bucket, "/\\ ") {
		return Resolved{}, fmt.Errorf("staging_bucket must be an S3 bucket name, not a URI or path")
	}
	role := strings.TrimSpace(cfg.String("associated_iam_role"))
	if role == "" {
		role = "default"
	}
	if role != "default" && !strings.HasPrefix(role, "arn:") {
		return Resolved{}, fmt.Errorf("associated_iam_role must be an IAM role ARN or %q", "default")
	}
	return Resolved{
		DriverConfig:        pool,
		StagingBucket:       bucket,
		StagingBucketRegion: strings.TrimSpace(cfg.String("staging_bucket_region")),
		AssociatedIAMRole:   role,
	}, nil
}

func fieldsDSN(cfg filament.Config) (string, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return "", fmt.Errorf("host is required when connection_method is %q", dbconfig.MethodFields)
	}
	port := cfg.Int("port")
	if port == 0 {
		port = DefaultPort
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
		sslMode = "verify-full"
	}
	switch sslMode {
	case "disable", "require", "verify-ca", "verify-full":
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
