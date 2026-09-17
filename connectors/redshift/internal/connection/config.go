package connection

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	// DefaultPort is the conventional Redshift database port.
	DefaultPort = 5439
	// DefaultSchema is the conventional Redshift destination schema.
	DefaultSchema = "public"
	// DefaultStagingPrefix keeps Filament's transient load objects together.
	DefaultStagingPrefix = "filament/load-staging"
)

// Fields returns the connection-scoped Redshift and S3 load-staging fields.
func Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return []filament.ConfigField{
		dbconfig.MethodConfigField(),
		dbconfig.DSNConfigField("Amazon Redshift connection URL"),
		{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Redshift provisioned-cluster or Serverless workgroup endpoint"},
		{Name: "port", Type: filament.FieldInt, Default: DefaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Redshift database port"},
		{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Redshift database user"},
		{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Redshift database password"},
		{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Redshift database to connect to"},
		{Name: "ssl_mode", Type: filament.FieldEnum, Default: "verify-full", Enum: []filament.EnumOption{
			{Value: "disable", Label: "Disable"},
			{Value: "require", Label: "Require"},
			{Value: "verify-ca", Label: "Verify CA"},
			{Value: "verify-full", Label: "Verify Full"},
		}, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "TLS verification mode"},
		{Name: "schema_name", Type: filament.FieldString, Default: DefaultSchema, Scope: filament.ScopeConnection, Help: "Default destination schema; pipelines may override it"},
		{Name: "staging_bucket", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "S3 bucket used for transient Parquet files and COPY manifests"},
		{Name: "staging_bucket_region", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "AWS region of the staging bucket; it must match the Redshift database region"},
		{Name: "associated_iam_role", Type: filament.FieldString, Required: true, Default: "default", Scope: filament.ScopeConnection, Help: "Associated IAM role ARN Redshift assumes for COPY, or default to use the cluster's default role"},
	}
}
