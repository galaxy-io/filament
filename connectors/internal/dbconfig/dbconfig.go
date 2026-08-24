// Package dbconfig contains the connection-input convention shared by database
// connectors. Individual connectors remain responsible for translating the
// selected representation into their driver's native configuration.
package dbconfig

import (
	"fmt"

	"github.com/galaxy-io/filament"
)

const (
	// MethodField is the config field selecting the connection representation.
	MethodField = "connection_method"
	// DSNField is the config field containing a connection URL or driver DSN.
	DSNField = "dsn"

	// MethodFields selects individually configured connection fields.
	MethodFields = "fields"
	// MethodURL selects a connection URL or driver DSN.
	MethodURL = "url"
)

// Method returns the selected connection representation. Fields is the default
// for new configurations; a missing method with a DSN remains URL mode for
// backward compatibility with connections created before the selector existed.
func Method(cfg filament.Config) (string, error) {
	method := cfg.String(MethodField)
	if method == "" {
		if cfg.Secret(DSNField) != "" {
			return MethodURL, nil
		}
		return MethodFields, nil
	}
	switch method {
	case MethodFields, MethodURL:
		return method, nil
	default:
		return "", fmt.Errorf("%s must be %q or %q", MethodField, MethodFields, MethodURL)
	}
}

// MethodConfigField declares the shared fields-first selector.
func MethodConfigField() filament.ConfigField {
	return filament.ConfigField{
		Name: MethodField, Type: filament.FieldEnum, Default: MethodFields,
		Enum: []filament.EnumOption{
			{Value: MethodFields, Label: "Details"},
			{Value: MethodURL, Label: "URL"},
		},
		Scope: filament.ScopeConnection,
		Help:  "Supply connection details as individual fields or as a connection URL (DSN)",
	}
}

// VisibleWhen returns a condition for a field belonging to one input method.
func VisibleWhen(method string) *filament.FieldCondition {
	return &filament.FieldCondition{Field: MethodField, Values: []string{method}}
}

// DSNConfigField declares the URL-mode secret.
func DSNConfigField(help string) filament.ConfigField {
	return filament.ConfigField{
		Name: DSNField, Type: filament.FieldSecret, Required: true,
		Scope: filament.ScopeConnection, Help: help, VisibleWhen: VisibleWhen(MethodURL),
	}
}
