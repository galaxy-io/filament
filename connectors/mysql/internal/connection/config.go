package connection

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const defaultPort = 3306

// Fields returns the connection-scoped configuration shared by the MySQL
// source and sink.
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
