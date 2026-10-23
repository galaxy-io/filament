package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Supported grant types and token endpoint client authentication methods.
const (
	ClientCredentials = "client_credentials"
	RefreshToken      = "refresh_token"
	ClientSecretPost  = "client_secret_post"
	ClientSecretBasic = "client_secret_basic"
)

// Config is immutable token endpoint configuration. Params adds provider
// parameters such as audience. Credentials and parameters are literal values,
// not templates. HTTPClient optionally supplies a transport (for example TLS).
type Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	ClientAuth   string
	GrantType    string
	Scope        string
	Params       map[string]string
	HTTPClient   *http.Client
}

// Source refreshes on demand. It is safe for concurrent use. There is no
// background goroutine and no automatic retry of token endpoint requests.
type Source struct {
	config Config
	store  Store
	lock   chan struct{}
	now    func() time.Time
	// Keep a successful exchange if persistence fails. A subsequent call
	// retries saving it rather than consuming the old refresh token again.
	pending *pendingToken
}

type pendingToken struct {
	previous Token
	next     Token
}

var _ TokenSource = (*Source)(nil)

// NewSource constructs a token source. A refresh-token source requires a
// Store seeded with a refresh token. A nil store is allowed for client
// credentials and creates a private in-memory cache.
func NewSource(config Config, store Store) (*Source, error) {
	u, err := url.Parse(config.TokenURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("oauth: token URL requires HTTPS without userinfo or fragment")
	}
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, errors.New("oauth: client ID and secret are required")
	}
	if config.GrantType == "" {
		config.GrantType = ClientCredentials
	}
	if config.GrantType != ClientCredentials && config.GrantType != RefreshToken {
		return nil, errors.New("oauth: unsupported grant type")
	}
	if config.ClientAuth == "" {
		config.ClientAuth = ClientSecretPost
	}
	if config.ClientAuth != ClientSecretPost && config.ClientAuth != ClientSecretBasic {
		return nil, errors.New("oauth: unsupported client authentication method")
	}
	params := make(map[string]string, len(config.Params))
	for key, value := range config.Params {
		switch key {
		case "grant_type", "client_id", "client_secret", "refresh_token", "scope":
			return nil, fmt.Errorf("oauth: reserved parameter %s", key)
		}
		params[key] = value
	}
	config.Params = params
	client := http.Client{Timeout: 30 * time.Second}
	if config.HTTPClient != nil {
		client = *config.HTTPClient
		if client.Timeout == 0 {
			client.Timeout = 30 * time.Second
		}
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	config.HTTPClient = &client
	if store == nil {
		if config.GrantType == RefreshToken {
			return nil, errors.New("oauth: refresh-token grant requires a token store")
		}
		store = NewMemoryStore(Token{})
	}
	return &Source{config: config, store: store, lock: make(chan struct{}, 1), now: time.Now}, nil
}

// Token returns only committed token state. Store implementations must not
// log the token or any callback arguments.
func (s *Source) Token(ctx context.Context) (Token, error) {
	return s.token(ctx, "")
}

// Refresh replaces an access token rejected by the API. If another caller has
// already saved a fresh replacement, it reuses that token instead of rotating
// again. The connector is responsible for bounding API request replays.
func (s *Source) Refresh(ctx context.Context, rejected string) (Token, error) {
	if rejected == "" {
		return Token{}, errors.New("oauth: rejected access token is required")
	}
	return s.token(ctx, rejected)
}

func (s *Source) token(ctx context.Context, rejected string) (Token, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}
	// A retained exchange may have expired while persistence was unavailable.
	// Commit its refresh token first, then acquire a usable access token.
	for range 2 {
		token, err := s.store.Update(ctx, func(ctx context.Context, current Token) (Token, error) {
			return s.refresh(ctx, current, rejected)
		})
		if err != nil {
			return Token{}, err
		}
		s.pending = nil
		if s.now().Before(token.ExpiresAt) {
			return token, nil
		}
	}
	return Token{}, errors.New("oauth: token expired before it could be used")
}

func (s *Source) refresh(ctx context.Context, current Token, rejected string) (Token, error) {
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}
	if s.pending != nil {
		if current == s.pending.previous {
			return s.pending.next, nil
		}
		// Another owner either committed this response or replaced the
		// credential. Never overwrite its newer state with ours.
		s.pending = nil
	}
	if current.AccessToken != "" && current.AccessToken != rejected && s.now().Add(30*time.Second).Before(current.ExpiresAt) {
		return current, nil
	}
	if s.config.GrantType == RefreshToken && current.RefreshToken == "" {
		return Token{}, ErrReauthorizationRequired
	}
	next, err := s.exchange(ctx, current)
	if err != nil {
		return Token{}, err
	}
	s.pending = &pendingToken{previous: current, next: next}
	return next, nil
}

func (s *Source) form(current Token) url.Values {
	c := s.config
	form := url.Values{"grant_type": {c.GrantType}}
	if c.ClientAuth == ClientSecretPost {
		form.Set("client_id", c.ClientID)
		form.Set("client_secret", c.ClientSecret)
	}
	if c.GrantType == RefreshToken {
		form.Set("refresh_token", current.RefreshToken)
	}
	if c.Scope != "" {
		form.Set("scope", c.Scope)
	}
	for key, value := range c.Params {
		form.Set(key, value)
	}
	return form
}

// SetAuthHeader applies a token to a request. Non-Bearer schemes belong to the
// connector adapter, which may use Token.AccessToken with its own prefix.
func (t Token) SetAuthHeader(req *http.Request) {
	kind := t.TokenType
	if kind == "" || strings.EqualFold(kind, "bearer") {
		kind = "Bearer"
	}
	req.Header.Set("Authorization", kind+" "+t.AccessToken)
}
