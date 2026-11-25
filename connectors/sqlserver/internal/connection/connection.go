// Package connection resolves SQL Server connection settings into a
// go-mssqldb connection URL, shared by every SQL Server reader.
package connection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/microsoft/go-mssqldb/azuread"
	"github.com/microsoft/go-mssqldb/msdsn"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	defaultPort = 1433

	// defaultPacketSize is the largest TDS packet the protocol allows. Bulk
	// reads spend less time on packet headers and reader wakeups with it, and
	// the server lowers it if configured to.
	defaultPacketSize = 32767

	// defaultConnectionTimeout bounds opening a connection, the login
	// included, in seconds.
	defaultConnectionTimeout = 30

	appName = "filament"
)

// Encryption modes, as go-mssqldb's encrypt parameter spells them.
const (
	EncryptTrue    = "true"    // TLS for the whole session
	EncryptStrict  = "strict"  // TDS 8.0: TLS before any TDS traffic
	EncryptFalse   = "false"   // TLS for the login packet only
	EncryptDisable = "disable" // no TLS at all
)

// Authentication types, the auth object's type field.
const (
	AuthSQL              = "sql"               // SQL Server login and password
	AuthServicePrincipal = "service_principal" // Microsoft Entra application and client secret
	AuthManagedIdentity  = "managed_identity"  // Microsoft Entra managed identity of the host
	AuthDefault          = "default"           // Azure default credential chain
)

// Profile is the deployment a source connects to. Both speak the same TDS
// protocol and catalog, so they share one reader. They differ only in how a
// connection is addressed, secured and signed in.
type Profile int

const (
	// SQLServer is a self-managed SQL Server, on premises or on a VM. It may
	// run as a named instance and present a self-signed certificate.
	SQLServer Profile = iota
	// AzureSQL is Azure SQL Database or Azure SQL Managed Instance. Its
	// endpoints always encrypt with a CA-signed certificate and accept
	// Microsoft Entra sign-in.
	AzureSQL
)

// Name is the display name of the deployment, used in field help.
func (p Profile) Name() string {
	if p == AzureSQL {
		return "Azure SQL"
	}
	return "SQL Server"
}

func (p Profile) encryptModes() []filament.EnumOption {
	modes := []filament.EnumOption{
		{Value: EncryptTrue, Label: "Require"},
		{Value: EncryptStrict, Label: "Strict (TDS 8.0)"},
	}
	if p == SQLServer {
		modes = append(modes,
			filament.EnumOption{Value: EncryptFalse, Label: "Login only"},
			filament.EnumOption{Value: EncryptDisable, Label: "Disable"},
		)
	}
	return modes
}

func (p Profile) authTypes() []filament.EnumOption {
	types := []filament.EnumOption{{Value: AuthSQL, Label: "SQL login"}}
	if p == AzureSQL {
		types = append(types,
			filament.EnumOption{Value: AuthServicePrincipal, Label: "Entra service principal"},
			filament.EnumOption{Value: AuthManagedIdentity, Label: "Entra managed identity"},
			filament.EnumOption{Value: AuthDefault, Label: "Entra default credentials"},
		)
	}
	return types
}

func allowed(options []filament.EnumOption, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

// Fields returns the connection-scoped configuration of a source for p.
func (p Profile) Fields() []filament.ConfigField {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	out := []filament.ConfigField{dbconfig.MethodConfigField()}
	if p == AzureSQL {
		out = append(out,
			dbconfig.DSNConfigField("Azure SQL URL, such as sqlserver://user:password@myserver.database.windows.net:1433?database=app"),
			filament.ConfigField{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Server name, such as myserver.database.windows.net"},
			filament.ConfigField{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Server port. A Managed Instance public endpoint listens on 3342"},
		)
	} else {
		out = append(out,
			dbconfig.DSNConfigField("SQL Server URL, such as sqlserver://user:password@host:1433?database=app"),
			filament.ConfigField{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "SQL Server hostname"},
			filament.ConfigField{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "SQL Server port"},
			filament.ConfigField{Name: "instance", Type: filament.FieldString, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Named instance, resolved through the SQL Server Browser service instead of the port"},
		)
	}
	out = append(out,
		p.authField(fields),
		filament.ConfigField{Name: "database_name", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Database to connect to"},
		filament.ConfigField{Name: "encrypt", Type: filament.FieldEnum, Default: EncryptTrue, Enum: p.encryptModes(), Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Connection encryption"},
	)
	if p == SQLServer {
		out = append(out, filament.ConfigField{Name: "trust_server_certificate", Type: filament.FieldBool, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Accept the server certificate without verifying it"})
	}
	return out
}

func (p Profile) authField(visible *filament.FieldCondition) filament.ConfigField {
	when := func(types ...string) *filament.FieldCondition {
		return &filament.FieldCondition{Field: "type", Values: types}
	}
	fields := []filament.ConfigField{
		{Name: "type", Type: filament.FieldEnum, Required: true, Default: AuthSQL, Help: "Authentication method", Enum: p.authTypes()},
		{Name: "username", Type: filament.FieldString, Required: true, VisibleWhen: when(AuthSQL), Help: p.Name() + " login"},
		{Name: "password", Type: filament.FieldSecret, Required: true, VisibleWhen: when(AuthSQL), Help: p.Name() + " password"},
	}
	if p == AzureSQL {
		fields = append(fields,
			filament.ConfigField{Name: "tenant_id", Type: filament.FieldString, Required: true, VisibleWhen: when(AuthServicePrincipal), Help: "Directory (tenant) ID of the application"},
			filament.ConfigField{Name: "client_id", Type: filament.FieldString, Required: true, VisibleWhen: when(AuthServicePrincipal), Help: "Application (client) ID"},
			filament.ConfigField{Name: "client_secret", Type: filament.FieldSecret, Required: true, VisibleWhen: when(AuthServicePrincipal), Help: "Application client secret"},
			filament.ConfigField{Name: "client_id", Type: filament.FieldString, VisibleWhen: when(AuthManagedIdentity), Help: "Client ID of a user-assigned identity. Empty uses the system-assigned identity"},
		)
	}
	return filament.ConfigField{
		Name: "auth", Type: filament.FieldObject, Required: true, Scope: filament.ScopeConnection,
		Default: map[string]any{"type": AuthSQL}, VisibleWhen: visible,
		Help:   "How Filament signs in to " + p.Name(),
		Fields: fields,
	}
}

// Resolved is a validated go-mssqldb connection URL.
type Resolved struct {
	URL      *url.URL
	Database string
}

// Resolve translates connector configuration for p into a go-mssqldb
// connection URL.
func (p Profile) Resolve(cfg filament.Config) (Resolved, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return Resolved{}, err
	}
	var u *url.URL
	if method == dbconfig.MethodURL {
		if u, err = parseURL(cfg.Secret(dbconfig.DSNField)); err != nil {
			return Resolved{}, err
		}
	} else if u, err = p.fromFields(cfg); err != nil {
		return Resolved{}, err
	}
	// Connection string keys are case-insensitive, and the driver rejects a
	// key given twice, so a default is added only when no casing of its key
	// is present. A URL without an encrypt setting gets the verified TLS that
	// fields mode defaults to, rather than the driver's login-only default,
	// which skips certificate checks. A connection timeout bounds the login
	// exchange: without one the driver bounds only the dial, so a server that
	// accepts the connection and never answers holds it past any deadline.
	q := u.Query()
	for key, value := range map[string]string{
		"packet size":        strconv.Itoa(defaultPacketSize),
		"app name":           appName,
		"encrypt":            EncryptTrue,
		"connection timeout": strconv.Itoa(defaultConnectionTimeout),
	} {
		if !hasKey(q, key) {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	// Let the driver reject what it would reject at connect time, now,
	// including an incomplete fedauth configuration.
	parsed, err := msdsn.Parse(u.String())
	if err != nil {
		return Resolved{}, fmt.Errorf("parse connection: %w", err)
	}
	if _, err := azuread.NewConnector(u.String()); err != nil {
		return Resolved{}, fmt.Errorf("parse connection: %w", err)
	}
	return Resolved{URL: u, Database: parsed.Database}, nil
}

// hasKey reports whether q sets key in any casing.
func hasKey(q url.Values, key string) bool {
	for k := range q {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func (p Profile) fromFields(cfg filament.Config) (*url.URL, error) {
	host := strings.TrimSpace(cfg.String("host"))
	if host == "" {
		return nil, fmt.Errorf("host is required when connection_method is %q", dbconfig.MethodFields)
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}
	database := cfg.String("database_name")
	if database == "" {
		return nil, fmt.Errorf("database_name is required when connection_method is %q", dbconfig.MethodFields)
	}
	encrypt := cfg.String("encrypt")
	if encrypt == "" {
		encrypt = EncryptTrue
	}
	if !allowed(p.encryptModes(), encrypt) {
		return nil, fmt.Errorf("unsupported encrypt %q for %s", encrypt, p.Name())
	}

	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	u := &url.URL{Scheme: "sqlserver"}
	if instance := strings.TrimSpace(cfg.String("instance")); instance != "" && p == SQLServer {
		// A named instance is found through the Browser service, not a port.
		u.Host, u.Path = host, "/"+instance
	} else {
		u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	}
	q := url.Values{}
	if err := p.applyAuth(u, q, cfg.Sub("auth")); err != nil {
		return nil, err
	}
	q.Set("database", database)
	q.Set("encrypt", encrypt)
	if p == SQLServer && cfg.Bool("trust_server_certificate") {
		q.Set("TrustServerCertificate", "true")
	}
	u.RawQuery = q.Encode()
	return u, nil
}

// applyAuth sets the login of a fields-mode URL. Entra methods select a
// go-mssqldb fedauth workflow, which the azuread connector turns into a token.
func (p Profile) applyAuth(u *url.URL, q url.Values, cfg filament.Config) error {
	authType := cfg.String("type")
	if authType == "" {
		authType = AuthSQL
	}
	if !allowed(p.authTypes(), authType) {
		return fmt.Errorf("unsupported auth.type %q for %s", authType, p.Name())
	}
	switch authType {
	case AuthSQL:
		username, password := cfg.String("username"), cfg.Secret("password")
		if username == "" || password == "" {
			return fmt.Errorf("auth.username and auth.password are required when auth.type is %q", AuthSQL)
		}
		u.User = url.UserPassword(username, password)
	case AuthServicePrincipal:
		tenant, client := strings.TrimSpace(cfg.String("tenant_id")), strings.TrimSpace(cfg.String("client_id"))
		secret := cfg.Secret("client_secret")
		if tenant == "" || client == "" || secret == "" {
			return fmt.Errorf("auth.tenant_id, auth.client_id and auth.client_secret are required when auth.type is %q", AuthServicePrincipal)
		}
		u.User = url.UserPassword(client+"@"+tenant, secret)
		q.Set("fedauth", azuread.ActiveDirectoryServicePrincipal)
	case AuthManagedIdentity:
		if client := strings.TrimSpace(cfg.String("client_id")); client != "" {
			u.User = url.User(client)
		}
		q.Set("fedauth", azuread.ActiveDirectoryManagedIdentity)
	case AuthDefault:
		q.Set("fedauth", azuread.ActiveDirectoryDefault)
	default:
		return fmt.Errorf("unsupported auth.type %q", authType)
	}
	return nil
}

func parseURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		// url.Error quotes the input, which holds the password.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "sqlserver") {
		return nil, fmt.Errorf("dsn must be a sqlserver:// URL")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("dsn has no hostname")
	}
	return u, nil
}

// Open returns a pool over the resolved connection. The driver connects lazily.
// The azuread connector handles SQL logins too, and acquires a Microsoft Entra
// token when the URL names a fedauth workflow.
func Open(resolved Resolved) (*sql.DB, error) {
	connector, err := azuread.NewConnector(resolved.URL.String())
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

// Test opens, pings and closes a short-lived pool.
func Test(ctx context.Context, resolved Resolved) error {
	db, err := Open(resolved)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}
