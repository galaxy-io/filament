package connection

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	gosnowflake "github.com/snowflakedb/gosnowflake/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const application = "filament"

// Resolved contains the native Snowflake driver configuration.
type Resolved struct {
	DriverConfig *gosnowflake.Config
}

// Resolve translates connector configuration into Snowflake driver options.
func Resolve(cfg filament.Config) (Resolved, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return Resolved{}, err
	}
	if method == dbconfig.MethodURL {
		return resolveDSN(cfg)
	}
	return resolveFields(cfg)
}

func resolveDSN(cfg filament.Config) (Resolved, error) {
	dsn := cfg.Secret(dbconfig.DSNField)
	if strings.TrimSpace(dsn) == "" {
		return Resolved{}, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
	}
	driverConfig, err := gosnowflake.ParseDSN(dsn)
	if err != nil {
		// Driver parse errors may include fragments of the connection string.
		// Keep the public error useful without risking credential disclosure.
		return Resolved{}, fmt.Errorf("parse dsn: invalid Snowflake connection string")
	}
	if override := strings.TrimSpace(cfg.String("schema")); override != "" {
		driverConfig.Schema = override
	}
	if strings.TrimSpace(driverConfig.Schema) == "" {
		driverConfig.Schema = DefaultSchema
	}
	if err := validateDriverConfig(driverConfig); err != nil {
		return Resolved{}, err
	}
	driverConfig.Application = application
	return Resolved{DriverConfig: driverConfig}, nil
}

func resolveFields(cfg filament.Config) (Resolved, error) {
	account, err := requiredString(cfg, "account")
	if err != nil {
		return Resolved{}, err
	}
	username, err := requiredString(cfg, "username")
	if err != nil {
		return Resolved{}, err
	}
	warehouse, err := requiredString(cfg, "warehouse")
	if err != nil {
		return Resolved{}, err
	}
	database, err := requiredString(cfg, "database_name")
	if err != nil {
		return Resolved{}, err
	}
	schema := strings.TrimSpace(cfg.String("schema"))
	if schema == "" {
		schema = strings.TrimSpace(cfg.String("schema_name"))
	}
	if schema == "" {
		schema = DefaultSchema
	}

	driverConfig := &gosnowflake.Config{
		Account: account, User: username, Warehouse: warehouse,
		Database: database, Schema: schema, Role: strings.TrimSpace(cfg.String("role")),
		Application: application,
	}
	if err := applyAuth(driverConfig, cfg.Sub("auth")); err != nil {
		return Resolved{}, err
	}
	return Resolved{DriverConfig: driverConfig}, nil
}

func requiredString(cfg filament.Config, field string) (string, error) {
	value := strings.TrimSpace(cfg.String(field))
	if value == "" {
		return "", fmt.Errorf("%s is required when connection_method is %q", field, dbconfig.MethodFields)
	}
	return value, nil
}

func applyAuth(driverConfig *gosnowflake.Config, cfg filament.Config) error {
	authType := cfg.String("type")
	if authType == "" {
		authType = authKeyPair
	}
	switch authType {
	case authKeyPair:
		raw := cfg.Secret("private_key")
		if strings.TrimSpace(raw) == "" {
			return fmt.Errorf("auth.private_key is required when auth.type is %q", authKeyPair)
		}
		key, err := parsePrivateKey([]byte(raw))
		if err != nil {
			return fmt.Errorf("auth.private_key: %w", err)
		}
		driverConfig.Authenticator = gosnowflake.AuthTypeJwt
		driverConfig.PrivateKey = key
	case authPAT:
		token := cfg.Secret("token")
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("auth.token is required when auth.type is %q", authPAT)
		}
		driverConfig.Authenticator = gosnowflake.AuthTypePat
		driverConfig.Token = token
	default:
		return fmt.Errorf("unsupported auth.type %q", authType)
	}
	return nil
}

func parsePrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("must contain an RSA private-key PEM block")
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, fmt.Errorf("must contain exactly one PEM block")
	}
	if strings.Contains(block.Type, "ENCRYPTED") || strings.Contains(strings.ToUpper(block.Headers["Proc-Type"]), "ENCRYPTED") {
		return nil, fmt.Errorf("encrypted PEM keys are not supported")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8 or PKCS#1 RSA key: invalid key data")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must be RSA")
	}
	return key, nil
}

func validateDriverConfig(cfg *gosnowflake.Config) error {
	if strings.TrimSpace(cfg.Account) == "" {
		return fmt.Errorf("dsn must specify an account")
	}
	if strings.TrimSpace(cfg.User) == "" {
		return fmt.Errorf("dsn must specify a username")
	}
	if strings.TrimSpace(cfg.Warehouse) == "" {
		return fmt.Errorf("dsn must specify a warehouse")
	}
	if strings.TrimSpace(cfg.Database) == "" {
		return fmt.Errorf("dsn must specify a database")
	}
	return nil
}
