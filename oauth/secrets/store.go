// Package secrets adapts Filament's encrypted secret providers to oauth.Store.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/oauth"
)

// updater is an optional secret-provider capability. The callback runs under
// an exclusive database lock, including across processes.
type updater interface {
	UpdateSecret(context.Context, filament.TenantID, string, func(filament.Secret) (filament.Secret, error)) (filament.Secret, error)
}

// Store owns one tenant's OAuth authorization, independently of connection or
// pipeline versions. Use the same ID wherever that authorization is shared.
type Store struct {
	provider filament.Secrets
	updater  updater
	tenant   filament.TenantID
	ref      string
	binding  string
}

var _ oauth.Store = (*Store)(nil)

// New binds an existing secret provider. Postgres and SQLite support atomic
// updates. Other providers are rejected rather than silently losing rotations.
func New(provider filament.Secrets, tenant filament.TenantID, id string) (*Store, error) {
	if tenant == "" || id == "" || strings.ContainsAny(string(tenant)+id, "/\\") {
		return nil, errors.New("oauth: tenant and credential ID must be nonempty path segments")
	}
	u, ok := provider.(updater)
	if !ok {
		return nil, errors.New("oauth: secret provider does not support atomic token updates")
	}
	return &Store{provider: provider, updater: u, tenant: tenant, ref: fmt.Sprintf("%s%s/oauth/%s", filament.ConnectionSecretPrefix, tenant, id)}, nil
}

// Set seeds or explicitly replaces an authorization, for example from a
// manually supplied refresh token or a future authorization-code exchange.
// The provider serializes replacement against an in-flight refresh.
func (s *Store) Set(ctx context.Context, token oauth.Token) error {
	if token.AccessToken == "" && token.RefreshToken == "" {
		return errors.New("oauth: a token is required")
	}
	secret, err := s.encode(filament.Secret{Tenant: s.tenant}, token)
	if err != nil {
		return errors.New("oauth: cannot encode token")
	}
	return s.provider.Write(ctx, s.ref, secret)
}

// Delete removes local credentials. Provider-side revocation is separate.
func (s *Store) Delete(ctx context.Context) error { return s.provider.Delete(ctx, s.ref) }

// Update runs fn under the provider's database lock and commits encrypted state.
func (s *Store) Update(ctx context.Context, fn func(context.Context, oauth.Token) (oauth.Token, error)) (oauth.Token, error) {
	// Keep database lock ownership bounded even when a caller has no deadline.
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	secret, err := s.updater.UpdateSecret(ctx, s.tenant, s.ref, func(secret filament.Secret) (filament.Secret, error) {
		current, err := s.decode(secret)
		if err != nil {
			return filament.Secret{}, err
		}
		next, err := fn(ctx, current)
		if err != nil {
			return filament.Secret{}, err
		}
		if next == current {
			return secret, nil
		}
		return s.encode(secret, next)
	})
	if err != nil {
		return oauth.Token{}, err
	}
	return s.decode(secret)
}

type boundToken struct {
	Token   oauth.Token `json:"token"`
	Binding string      `json:"binding"`
}

func (s *Store) decode(secret filament.Secret) (oauth.Token, error) {
	if s.binding != "" {
		switch secret.Meta["oauth_format"] {
		case "":
			return oauth.Token{RefreshToken: string(secret.Value)}, nil
		case "v1":
			var state boundToken
			if err := json.Unmarshal(secret.Value, &state); err != nil {
				return oauth.Token{}, errors.New("oauth: cannot decode stored token")
			}
			if state.Binding != s.binding {
				return oauth.Token{}, errors.New("oauth: authorization settings changed; supply a fresh refresh token")
			}
			return state.Token, nil
		default:
			return oauth.Token{}, errors.New("oauth: unsupported stored token format")
		}
	}
	var token oauth.Token
	if err := json.Unmarshal(secret.Value, &token); err != nil {
		return oauth.Token{}, errors.New("oauth: cannot decode stored token")
	}
	return token, nil
}

func (s *Store) encode(secret filament.Secret, token oauth.Token) (filament.Secret, error) {
	var value any = token
	if s.binding != "" {
		value = boundToken{Token: token, Binding: s.binding}
		if secret.Meta == nil {
			secret.Meta = map[string]string{}
		}
		secret.Meta["oauth_format"] = "v1"
	}
	data, err := json.Marshal(value) // #nosec G117 -- token payload goes only to the encrypted secret provider.
	if err != nil {
		return filament.Secret{}, errors.New("oauth: cannot encode token")
	}
	secret.Value = data
	return secret, nil
}
