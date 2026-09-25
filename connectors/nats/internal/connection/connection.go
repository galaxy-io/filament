// Package connection holds the connection settings, validation, and dialing
// shared by the NATS JetStream source and sink.
package connection

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

// Catalog identity shared by both connectors.
const (
	DisplayName  = "NATS JetStream"
	DarkLogoURL  = "https://cdn.getgalaxy.io/sources/source-icon-nats-dark.svg"
	LightLogoURL = "https://cdn.getgalaxy.io/sources/source-icon-nats-light.svg"
)

const dialTimeout = 5 * time.Second

// Fields returns the connection-scoped configuration shared by the source and sink.
func Fields() []filament.ConfigField {
	visible := func(mode string) *filament.FieldCondition {
		return &filament.FieldCondition{Field: "auth_mode", Values: []string{mode}}
	}
	return []filament.ConfigField{
		{Name: "url", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "NATS server URL; use tls:// for TLS"},
		{Name: "auth_mode", Type: filament.FieldEnum, Default: "none", Scope: filament.ScopeConnection, Enum: []filament.EnumOption{{Value: "none", Label: "None"}, {Value: "user_password", Label: "Username / Password"}, {Value: "jwt", Label: "JWT / NKey Seed"}, {Value: "token", Label: "Token"}}},
		{Name: "username", Required: true, VisibleWhen: visible("user_password"), Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "NATS username; supply with password"},
		{Name: "password", Required: true, VisibleWhen: visible("user_password"), Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "NATS password; supply with username"},
		{Name: "jwt", Required: true, VisibleWhen: visible("jwt"), Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "NATS user JWT; supply with its matching NKey seed"},
		{Name: "nkey_seed", Required: true, VisibleWhen: visible("jwt"), Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "Private user NKey seed for JWT authentication"},
		{Name: "token", Required: true, VisibleWhen: visible("token"), Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "Authentication token; cannot be combined with other authentication methods"},
	}
}

// Validate checks connection settings independently of pipeline settings.
func Validate(cfg filament.Config) error {
	if cfg.String("url") == "" {
		return errors.New("nats: url is required")
	}
	user, password := cfg.String("username"), cfg.Secret("password")
	jwt, seed := cfg.Secret("jwt"), cfg.Secret("nkey_seed")
	if (user == "") != (password == "") {
		return errors.New("nats: username and password must be supplied together")
	}
	if (jwt == "") != (seed == "") {
		return errors.New("nats: jwt and nkey_seed must be supplied together")
	}
	methods := 0
	for _, enabled := range []bool{user != "", jwt != "", cfg.Secret("token") != ""} {
		if enabled {
			methods++
		}
	}
	if methods > 1 {
		return errors.New("nats: choose only one authentication method: username/password, jwt/nkey_seed, or token")
	}
	mode := cfg.String("auth_mode")
	switch mode {
	case "", "none":
		if methods != 0 {
			return errors.New("nats: auth_mode none cannot include credentials")
		}
	case "user_password":
		if user == "" {
			return errors.New("nats: username/password authentication requires username and password")
		}
	case "jwt":
		if jwt == "" {
			return errors.New("nats: JWT authentication requires jwt and nkey_seed")
		}
	case "token":
		if cfg.Secret("token") == "" {
			return errors.New("nats: token authentication requires token")
		}
	default:
		return errors.New("nats: unknown auth_mode")
	}
	return nil
}

// Connect dials one connection without reconnection. The dial timeout never
// outlives the context deadline, and cancellation before return closes it.
func Connect(ctx context.Context, cfg filament.Config) (*nats.Conn, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := dialTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	opts := []nats.Option{nats.Timeout(timeout), nats.NoReconnect()}
	opts = append(opts, authOptions(cfg)...)
	conn, err := nats.Connect(cfg.String("url"), opts...)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func authOptions(cfg filament.Config) []nats.Option {
	switch cfg.String("auth_mode") {
	case "user_password":
		return []nats.Option{nats.UserInfo(cfg.String("username"), cfg.Secret("password"))}
	case "jwt":
		return []nats.Option{nats.UserJWTAndSeed(cfg.Secret("jwt"), cfg.Secret("nkey_seed"))}
	case "token":
		return []nats.Option{nats.Token(cfg.Secret("token"))}
	default:
		return nil
	}
}
