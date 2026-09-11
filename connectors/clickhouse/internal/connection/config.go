package connection

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	// DefaultDatabase is the always-present database used to bootstrap connections.
	DefaultDatabase = "default"
	defaultPort     = 9440
	defaultUsername = "default"
	protocolNative  = "native"
	protocolHTTP    = "http"
)

// Fields returns the connection-scoped configuration for ClickHouse.
func Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("ClickHouse connection URL"),
		{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse server hostname; copied http:// or https:// endpoints are also accepted"},
		{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse server port (9440 for secure native connections)"},
		{Name: "protocol", Type: filament.FieldEnum, Default: protocolNative, Enum: []filament.EnumOption{{Value: protocolNative, Label: "Native"}, {Value: protocolHTTP, Label: "HTTP"}}, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse wire protocol"},
		{Name: "username", Type: filament.FieldString, Default: defaultUsername, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse username"},
		{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse password"},
		{Name: "database_name", Type: filament.FieldString, Default: DefaultDatabase, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Database encoded in the connection; pipeline configuration selects the destination database"},
		{Name: "secure", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Connect with TLS (required by ClickHouse Cloud)"},
	}
}
