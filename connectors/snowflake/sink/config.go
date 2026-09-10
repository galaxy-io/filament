// Package connection owns the Snowflake connection configuration surface and
// translates Filament configuration into the official driver's native config.
package snowflake

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"strings"

	gosnowflake "github.com/snowflakedb/gosnowflake/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	defaultSchema = "PUBLIC"
	application   = "filament"

	authKeyPair  = "key_pair"
	authPAT      = "pat"
	authPassword = "password"
)

// resolvedConnection is a validated Snowflake driver configuration.
type resolvedConnection struct {
	driverConfig *gosnowflake.Config
}

// open returns a database handle backed by the official Snowflake connector.
// The handle does not establish a network connection until its first operation.
func (r resolvedConnection) open() *sql.DB {
	return sql.OpenDB(gosnowflake.NewConnector(gosnowflake.SnowflakeDriver{}, *r.driverConfig))
}

// connectionFields returns the reusable connection-level configuration fields. The
// destination schema is pipeline-scoped and is added by the sink spec.
func connectionFields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("Snowflake connection string"),
		{Name: "account", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Snowflake account identifier in organization-account form"},
		{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Snowflake service user"},
		{Name: "warehouse", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Virtual warehouse used for loading"},
		{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Destination database"},
		{Name: "schema_name", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Default schema used for connection tests; pipelines may override it"},
		{Name: "role", Type: filament.FieldString, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Optional Snowflake role"},
		authField(fields),
	}
}

func authField(visible *filament.FieldCondition) filament.ConfigField {
	return filament.ConfigField{
		Name: "auth", Type: filament.FieldObject, Required: true, Scope: filament.ScopeConnection,
		Default: map[string]any{"type": authKeyPair}, VisibleWhen: visible,
		Help: "Non-interactive authentication for the Snowflake service user",
		Fields: []filament.ConfigField{
			{Name: "type", Type: filament.FieldEnum, Required: true, Default: authKeyPair, Help: "Authentication method", Enum: []filament.EnumOption{
				{Value: authKeyPair, Label: "Key pair"},
				{Value: authPAT, Label: "Programmatic access token"},
				{Value: authPassword, Label: "Password"},
			}},
			{Name: "private_key", Type: filament.FieldSecret, Required: true, Help: "Unencrypted PEM-encoded RSA private key", VisibleWhen: authCondition(authKeyPair)},
			{Name: "token", Type: filament.FieldSecret, Required: true, Help: "Snowflake programmatic access token", VisibleWhen: authCondition(authPAT)},
			{Name: "password", Type: filament.FieldSecret, Required: true, Help: "Snowflake password", VisibleWhen: authCondition(authPassword)},
		},
	}
}

func authCondition(authType string) *filament.FieldCondition {
	return &filament.FieldCondition{Field: "type", Values: []string{authType}}
}

// resolveConnection translates connector configuration into the official driver's
// native Config. URL mode accepts any authentication supported by the driver;
// fields mode intentionally exposes only non-interactive sink credentials.
func resolveConnection(cfg filament.Config) (resolvedConnection, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return resolvedConnection{}, err
	}
	if method == dbconfig.MethodURL {
		return resolveDSN(cfg)
	}
	return resolveFields(cfg)
}

func resolveDSN(cfg filament.Config) (resolvedConnection, error) {
	dsn := cfg.Secret(dbconfig.DSNField)
	if strings.TrimSpace(dsn) == "" {
		return resolvedConnection{}, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
	}
	driverConfig, err := gosnowflake.ParseDSN(dsn)
	if err != nil {
		// Driver parse errors may include fragments of the connection string.
		// Keep the public error useful without risking credential disclosure.
		return resolvedConnection{}, fmt.Errorf("parse dsn: invalid Snowflake connection string")
	}
	if override := strings.TrimSpace(cfg.String("schema")); override != "" {
		driverConfig.Schema = override
	}
	if strings.TrimSpace(driverConfig.Schema) == "" {
		driverConfig.Schema = defaultSchema
	}
	if err := validateDriverConfig(driverConfig); err != nil {
		return resolvedConnection{}, err
	}
	driverConfig.Application = application
	return resolvedConnection{driverConfig: driverConfig}, nil
}

func resolveFields(cfg filament.Config) (resolvedConnection, error) {
	account, err := requiredString(cfg, "account")
	if err != nil {
		return resolvedConnection{}, err
	}
	username, err := requiredString(cfg, "username")
	if err != nil {
		return resolvedConnection{}, err
	}
	warehouse, err := requiredString(cfg, "warehouse")
	if err != nil {
		return resolvedConnection{}, err
	}
	database, err := requiredString(cfg, "database_name")
	if err != nil {
		return resolvedConnection{}, err
	}
	schema := strings.TrimSpace(cfg.String("schema"))
	if schema == "" {
		schema = strings.TrimSpace(cfg.String("schema_name"))
	}
	if schema == "" {
		schema = defaultSchema
	}

	driverConfig := &gosnowflake.Config{
		Account: account, User: username, Warehouse: warehouse,
		Database: database, Schema: schema, Role: strings.TrimSpace(cfg.String("role")),
		Application: application,
	}
	if err := applyAuth(driverConfig, cfg.Sub("auth")); err != nil {
		return resolvedConnection{}, err
	}
	return resolvedConnection{driverConfig: driverConfig}, nil
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
	case authPassword:
		password := cfg.Secret("password")
		if password == "" {
			return fmt.Errorf("auth.password is required when auth.type is %q", authPassword)
		}
		driverConfig.Authenticator = gosnowflake.AuthTypeSnowflake
		driverConfig.Password = password
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
