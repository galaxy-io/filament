// Package connection resolves MotherDuck connection configuration.
package connection

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/galaxy-io/filament"
)

// Resolved contains the validated values needed to open one DuckDB database.
type Resolved struct {
	Path    string
	Options map[string]string
}

// DSN renders the database path with its options as the driver's query string.
func (r Resolved) DSN() string {
	if len(r.Options) == 0 {
		return r.Path
	}
	values := make(url.Values, len(r.Options))
	for name, value := range r.Options {
		values.Set(name, value)
	}
	return r.Path + "?" + values.Encode()
}

// Fields returns MotherDuck's connection-scoped configuration.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{
		{Name: "token", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "MotherDuck service token"},
		{Name: "database", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Destination MotherDuck database"},
	}
}

// Resolve validates a token and the destination database.
func Resolve(cfg filament.Config) (Resolved, error) {
	token := cfg.Secret("token")
	if strings.TrimSpace(token) == "" {
		return Resolved{}, fmt.Errorf("token is required")
	}
	database := strings.TrimSpace(cfg.String("database"))
	if database == "" {
		return Resolved{}, fmt.Errorf("database is required")
	}
	return Resolved{
		Path:    "md:" + database,
		Options: map[string]string{"motherduck_token": token},
	}, nil
}
