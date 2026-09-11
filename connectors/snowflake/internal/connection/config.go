package connection

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	// DefaultSchema is Snowflake's conventional default schema.
	DefaultSchema = "PUBLIC"

	authKeyPair  = "key_pair"
	authPAT      = "pat"
	authPassword = "password"
)

// Fields returns the connection-scoped configuration for Snowflake.
func Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("Snowflake connection string"),
		{Name: "account", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Snowflake account identifier in organization-account form"},
		{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Snowflake service user"},
		{Name: "warehouse", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Virtual warehouse used for loading"},
		{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Destination database"},
		{Name: "schema_name", Type: filament.FieldString, Default: DefaultSchema, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Default schema used for connection tests; pipelines may override it"},
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
