// Package secrets adapts Filament's secret providers to a run-scoped OAuth store.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"maps"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/oauth"
)

// Store keeps access tokens in memory for one run and writes changed refresh
// tokens back to the same secret. It serializes callers within this store only.
type Store struct {
	provider filament.Secrets
	rotator  filament.SecretRotator
	tenant   filament.TenantID
	ref      string
	binding  string
	lock     chan struct{}
	token    oauth.Token
	loaded   bool
	secret   filament.Secret
	pending  *oauth.Token
}

var _ oauth.Store = (*Store)(nil)

// Update loads the refresh token once and serializes exchange and write-back.
// A failed write retains the successful exchange in memory for the next save
// attempt, without exchanging the consumed refresh token again.
func (s *Store) Update(ctx context.Context, fn func(context.Context, oauth.Token) (oauth.Token, error)) (oauth.Token, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	case <-ctx.Done():
		return oauth.Token{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return oauth.Token{}, err
	}
	current := s.token
	if !s.loaded {
		secret, err := s.provider.Read(ctx, s.ref)
		if err != nil {
			return oauth.Token{}, err
		}
		if secret.Tenant != "" && secret.Tenant != s.tenant {
			return oauth.Token{}, errors.New("oauth: secret belongs to another tenant")
		}
		secret.Tenant = s.tenant
		refresh, err := s.refreshToken(secret)
		if err != nil {
			return oauth.Token{}, err
		}
		current = oauth.Token{RefreshToken: refresh}
		secret.Value = []byte(refresh)
		secret.Meta = maps.Clone(secret.Meta)
		delete(secret.Meta, "oauth_format")
		s.secret = secret
		s.token = current
		s.loaded = true
	}
	if s.pending != nil {
		current = *s.pending
	} else {
		next, err := fn(ctx, current)
		if err != nil {
			return oauth.Token{}, err
		}
		current = next
		s.pending = &next
	}
	if current.RefreshToken != string(s.secret.Value) {
		next := s.secret
		next.Value = []byte(current.RefreshToken)
		if err := s.rotator.RotateSecret(ctx, s.ref, next); err != nil {
			return oauth.Token{}, err
		}
		s.secret = next
	}
	s.token = current
	s.pending = nil
	return current, nil
}

// Existing v1 payloads are read only to recover their refresh token. Persisted
// access tokens are never reused. The next rotation replaces v1 with plain bytes.
func (s *Store) refreshToken(secret filament.Secret) (string, error) {
	switch secret.Meta["oauth_format"] {
	case "":
		return string(secret.Value), nil
	case "v1":
		var state struct {
			Token   oauth.Token `json:"token"`
			Binding string      `json:"binding"`
		}
		if err := json.Unmarshal(secret.Value, &state); err != nil {
			return "", errors.New("oauth: cannot decode stored token")
		}
		if state.Binding != s.binding {
			return "", errors.New("oauth: authorization settings changed; supply a fresh refresh token")
		}
		return state.Token.RefreshToken, nil
	default:
		return "", errors.New("oauth: unsupported stored token format")
	}
}
