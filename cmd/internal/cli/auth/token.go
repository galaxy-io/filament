package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// expirySkew refreshes a cached token this long before it expires so a
// request never leaves with a token that dies in flight.
const expirySkew = time.Minute

// Source mints access tokens for one stored profile via the OAuth2 client
// credentials grant, reusing the cached token until it nears expiry.
type Source struct {
	Store   Store
	Profile string
	// Client issues discovery and token requests; nil means the default.
	Client *http.Client
	// Now is a clock override for tests; nil means time.Now.
	Now func() time.Time
}

// Mint exchanges an in-memory profile without persisting it. Login uses this
// to prove new credentials before replacing a working stored profile.
func Mint(ctx context.Context, profile Profile, client *http.Client) (string, Cache, error) {
	token, err := (Source{Client: client}).mint(ctx, profile)
	if err != nil {
		return "", Cache{}, err
	}
	return token.AccessToken, Cache{AccessToken: token.AccessToken, ExpiresAt: token.Expiry}, nil
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
	token, err := s.mint(ctx, profile)
	if err != nil {
		return "", err
	}
	cache := Cache{AccessToken: token.AccessToken, ExpiresAt: token.Expiry}
	if err := s.Store.SaveCache(s.Profile, cache); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// Invalidate discards the cached token after the server rejected it, so the
// caller's retry mints a fresh one.
func (s Source) Invalidate() error { return s.Store.ClearCache(s.Profile) }

func (s Source) mint(ctx context.Context, profile Profile) (*oauth2.Token, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	endpoint, err := tokenEndpoint(ctx, client, profile.Issuer)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	config := clientcredentials.Config{
		ClientID:     profile.ClientID,
		ClientSecret: profile.ClientSecret,
		TokenURL:     endpoint,
		Scopes:       profile.Scopes,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}
	token, err := config.Token(ctx)
	if err != nil {
		var refused *oauth2.RetrieveError
		if errors.As(err, &refused) {
			return nil, fmt.Errorf("%w: %w", ErrTokenRejected, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrAuthServerUnreachable, err)
	}
	return token, nil
}

// tokenEndpoint resolves the issuer's token endpoint through OIDC discovery,
// keeping the source agnostic to the identity provider behind the issuer.
func tokenEndpoint(ctx context.Context, client *http.Client, issuer string) (string, error) {
	url := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrAuthServerUnreachable, issuer, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %s: discovery returned status %d", ErrAuthServerInvalid, issuer, resp.StatusCode)
	}
	var discovered struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discovered); err != nil {
		return "", fmt.Errorf("%w: %s: discovery is not valid JSON: %w", ErrAuthServerInvalid, issuer, err)
	}
	if discovered.TokenEndpoint == "" {
		return "", fmt.Errorf("%w: %s: discovery lists no token endpoint", ErrAuthServerInvalid, issuer)
	}
	return discovered.TokenEndpoint, nil
}
