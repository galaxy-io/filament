package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

const (
	authNone              = "none"
	authBearer            = "bearer"
	authClientCredentials = "client_credentials"
)

func restAuthField(provider string) filament.ConfigField {
	return filament.ConfigField{
		Name:        "auth",
		Type:        filament.FieldObject,
		Help:        "Catalog authentication.",
		VisibleWhen: providerCondition(provider),
		Fields: []filament.ConfigField{
			{
				Name:    "type",
				Type:    filament.FieldEnum,
				Default: authNone,
				Enum: []filament.EnumOption{
					{Value: authNone, Label: "None"},
					{Value: authBearer, Label: "Bearer token"},
					{Value: authClientCredentials, Label: "Client credentials"},
				},
			},
			{Name: "token", Type: filament.FieldSecret, Secret: true, Required: true, VisibleWhen: authCondition(authBearer)},
			{Name: "client_id", Type: filament.FieldString, Required: true, VisibleWhen: authCondition(authClientCredentials)},
			{Name: "client_secret", Type: filament.FieldSecret, Secret: true, Required: true, VisibleWhen: authCondition(authClientCredentials)},
			{Name: "token_uri", Type: filament.FieldString, VisibleWhen: authCondition(authClientCredentials)},
			{Name: "scope", Type: filament.FieldString, VisibleWhen: authCondition(authClientCredentials)},
			{Name: "audience", Type: filament.FieldString, VisibleWhen: authCondition(authClientCredentials)},
			{Name: "resource", Type: filament.FieldString, VisibleWhen: authCondition(authClientCredentials)},
		},
	}
}

func authCondition(authType string) *filament.FieldCondition {
	return &filament.FieldCondition{Field: "type", Values: []string{authType}}
}

func applyRESTAuth(properties iceberg.Properties, cfg filament.Config) error {
	auth := cfg.Sub("auth")
	switch authType := auth.String("type"); authType {
	case "", authNone:
		return nil
	case authBearer:
		properties["token"] = auth.Secret("token")
	case authClientCredentials:
		properties["credential"] = fmt.Sprintf(
			"%s:%s",
			auth.String("client_id"),
			auth.Secret("client_secret"),
		)
		copyStringProperties(properties, auth, map[string]string{
			"token_uri": "rest.authorization-url",
			"scope":     "scope",
			"audience":  "audience",
			"resource":  "resource",
		})
	default:
		return fmt.Errorf("unsupported auth type %q", authType)
	}
	return nil
}

func copyStringProperties(dst iceberg.Properties, cfg filament.Config, fields map[string]string) {
	for field, property := range fields {
		if value := cfg.String(field); value != "" {
			dst[property] = value
		}
	}
}
