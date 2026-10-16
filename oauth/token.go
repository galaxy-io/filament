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

// Store owns the tokens for one authorization. Update must read the latest
// token, call fn exactly once, and commit its result before returning success.
// An error from fn must leave the stored token unchanged.
//
// Implementations must serialize the entire operation, including fn, against
// other refreshes and credential replacements for the same authorization.
// Durable stores must do this across processes, encrypt token material, and
// scope access to the tenant. A Load followed by an unconditional Save, or a
// compare-and-swap after fn, does not meet this contract: fn can rotate the
// upstream credential. Never automatically retry fn after a commit failure.
// Pass a context to fn that is canceled when transaction ownership ends.
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
// durable. Use a durable Store for providers with rotating refresh tokens.
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
