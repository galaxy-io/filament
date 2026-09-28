package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
)

// expirySkew refreshes a cached token this long before it expires so a
// request never leaves with a token that dies in flight.
const expirySkew = time.Minute

// Source mints access tokens for one stored profile through the server's
// GetToken RPC, reusing the cached token until it nears expiry.
type Source struct {
	Store   Store
	Profile string
	// Client carries the token requests; nil means the default.
	Client *http.Client
	// Now is a clock override for tests; nil means time.Now.
	Now func() time.Time
}

// Mint exchanges an in-memory profile without persisting it. Login uses this
// to prove new credentials before replacing a working stored profile.
func Mint(ctx context.Context, profile Profile, client *http.Client) (string, Cache, error) {
	cache, err := (Source{Client: client}).mint(ctx, profile)
	if err != nil {
		return "", Cache{}, err
	}
	return cache.AccessToken, cache, nil
}

// Token returns a valid access token, minting and persisting a fresh one
// when no usable cache exists.
func (s Source) Token(ctx context.Context) (string, error) {
	profile, err := s.Store.Get(s.Profile)
	if err != nil {
		return "", err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	if c := profile.Cache; c != nil && now().Add(expirySkew).Before(c.ExpiresAt) {
		return c.AccessToken, nil
	}
	cache, err := s.mint(ctx, profile)
	if err != nil {
		return "", err
	}
	if err := s.Store.SaveCache(s.Profile, cache); err != nil {
		return "", err
	}
	return cache.AccessToken, nil
}

// Invalidate discards the cached token after the server rejected it, so the
// caller's retry mints a fresh one.
func (s Source) Invalidate() error { return s.Store.ClearCache(s.Profile) }

func (s Source) mint(ctx context.Context, profile Profile) (Cache, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	auth := authv1connect.NewAuthServiceClient(client, profile.Server)
	res, err := auth.GetToken(ctx, connect.NewRequest(&authv1.GetTokenRequest{
		ClientId:     profile.ClientID,
		ClientSecret: profile.ClientSecret,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeUnauthenticated {
			return Cache{}, fmt.Errorf("%w: %w", ErrTokenRejected, err)
		}
		return Cache{}, fmt.Errorf("%w: %s: %w", ErrAuthServerUnreachable, profile.Server, err)
	}
	return Cache{
		AccessToken: res.Msg.GetAccessToken(),
		ExpiresAt:   now().Add(time.Duration(res.Msg.GetExpiresIn()) * time.Second),
	}, nil
}
