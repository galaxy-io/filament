package connection

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const defaultPort = 5432

// Fields returns the connection-scoped configuration shared by the PostgreSQL
// source and sink.
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
