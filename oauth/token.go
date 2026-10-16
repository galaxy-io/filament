// Package oauth provides OAuth token acquisition and refresh for connectors.
// It does not implement browser authorization or application login.
package oauth

import (
	"context"
	"time"
)

// Token is credential material. Keep it out of logs, events, and public config.
// ExpiresAt is the actual provider expiry, before the refresh safety margin.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenSource supplies a usable access token, refreshing it when necessary.
type TokenSource interface {
	Token(context.Context) (Token, error)
}

// Store owns run-scoped tokens for one authorization. Update serializes callers
// within the store, calls fn once, and commits its result before returning success.
// An error from fn leaves the token unchanged. Stores may retain a successful
// result after a write-back failure and return it on a later save attempt without
// calling fn again. Never repeat an exchange merely because persistence failed.
type Store interface {
	Update(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error)
}

// StoreFunc adapts an application's coordinated token transaction to Store.
type StoreFunc func(context.Context, func(context.Context, Token) (Token, error)) (Token, error)

// Update calls the supplied coordinated transaction.
func (f StoreFunc) Update(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
	return f(ctx, fn)
}

// MemoryStore shares token state between sources in one process. It is not
// durable. Use oauth/secrets for write-back of rotating refresh tokens.
type MemoryStore struct {
	lock  chan struct{}
	token Token
}

// NewMemoryStore seeds a process-local authorization.
func NewMemoryStore(initial Token) *MemoryStore {
	return &MemoryStore{lock: make(chan struct{}, 1), token: initial}
}

// Update serializes token access and honors cancellation while waiting.
func (s *MemoryStore) Update(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}
	next, err := fn(ctx, s.token)
	if err != nil {
		return Token{}, err
	}
	s.token = next
	return next, nil
}
