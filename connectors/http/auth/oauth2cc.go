package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/template"
	"github.com/galaxy-io/filament/oauth"
)

func init() { Register("oauth2_cc", newOAuth2CC) }

// oauth2CC adapts manifest templates to the shared OAuth token source.
// Configuration and the optional store must be set before the first Apply.
type oauth2CC struct {
	params       map[string]any
	headerPrefix string
	httpClient   *http.Client
	retry        RetryPolicy
	store        oauth.Store
	mu           sync.Mutex
	source       *oauth.Source
}

// WithOAuth2RetryPolicy attaches a retry policy before the first Apply.
// Only explicit temporary endpoint rejections are eligible for retry.
func WithOAuth2RetryPolicy(a Authenticator, p RetryPolicy) Authenticator {
	if oc, ok := a.(*oauth2CC); ok && p != nil {
		oc.retry = p
	}
	return a
}

// WithOAuthStore binds an externally managed token store before the first
// Apply. The store belongs to one authorization, shared by all its callers.
// A chain with multiple OAuth grants needs individually bound authenticators.
func WithOAuthStore(a Authenticator, store oauth.Store) error {
	if store == nil {
		return nil
	}
	var grants []*oauth2CC
	var visit func(Authenticator)
	visit = func(a Authenticator) {
		switch a := a.(type) {
		case *oauth2CC:
			grants = append(grants, a)
		case *chainAuth:
			for _, child := range a.steps {
				visit(child)
			}
		}
	}
	visit(a)
	if len(grants) != 1 {
		return fmt.Errorf("oauth: token store requires exactly one OAuth authenticator")
	}
	grants[0].store = store
	return nil
}

func newOAuth2CC(params map[string]any) (Authenticator, error) {
	for _, key := range []string{"token_url", "client_id", "client_secret"} {
		if _, err := requireStrParam(params, "oauth2_cc", key); err != nil {
			return nil, err
		}
	}
	grant := strParam(params, "grant_type")
	if grant != "" && grant != oauth.ClientCredentials && grant != oauth.RefreshToken {
		return nil, fmt.Errorf("oauth2: unsupported grant_type")
	}
	if grant == oauth.RefreshToken && strParam(params, "refresh_token") == "" {
		return nil, fmt.Errorf("oauth2: refresh_token is required")
	}
	style := strParam(params, "client_auth")
	if style != "" && style != oauth.ClientSecretPost && style != oauth.ClientSecretBasic {
		return nil, fmt.Errorf("oauth2: unsupported client_auth")
	}
	extra, _ := params["params"].(map[string]any)
	for key, value := range extra {
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("oauth2: parameter %s must be a string", key)
		}
		switch key {
		case "grant_type", "client_id", "client_secret", "refresh_token", "scope":
			return nil, fmt.Errorf("oauth2: reserved parameter %s", key)
		}
	}
	prefix := strParam(params, "header_prefix")
	if prefix == "" {
		prefix = "Bearer"
	}
	return &oauth2CC{params: params, headerPrefix: prefix, retry: NoRetry{}}, nil
}

func (a *oauth2CC) Apply(ctx context.Context, req *http.Request, scope template.Scope) error {
	source, err := a.tokenSource(scope)
	if err != nil {
		return fmt.Errorf("%w: %w", errs.ErrAuthRefresh, err)
	}
	for attempt := 1; ; attempt++ {
		token, err := source.Token(ctx)
		if err == nil {
			req.Header.Set("Authorization", a.headerPrefix+" "+token.AccessToken)
			return nil
		}
		wrapped := fmt.Errorf("%w: %w", errs.ErrAuthRefresh, err)
		var endpoint *oauth.EndpointError
		if !errors.As(err, &endpoint) || !endpoint.Retryable() {
			return wrapped
		}
		delay, retry := a.retry.ShouldRetry(wrapped, attempt)
		if !retry {
			return wrapped
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (a *oauth2CC) DeclaredWrites() []Write {
	return []Write{{Kind: WriteHeader, Name: "Authorization"}}
}

func (a *oauth2CC) tokenSource(scope template.Scope) (*oauth.Source, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.source != nil {
		return a.source, nil
	}
	rendered := make(map[string]string)
	for _, key := range []string{"token_url", "client_id", "client_secret", "scope", "refresh_token"} {
		value, err := template.Render(strParam(a.params, key), scope)
		if err != nil {
			return nil, fmt.Errorf("oauth2: render %s", key)
		}
		rendered[key] = value
	}
	extra := make(map[string]string)
	rawExtra, _ := a.params["params"].(map[string]any)
	for key, value := range rawExtra {
		rendered, err := template.Render(value.(string), scope)
		if err != nil {
			return nil, fmt.Errorf("oauth2: render parameter %s", key)
		}
		extra[key] = rendered
	}
	store := a.store
	if store == nil && a.params["managed_refresh"] == true {
		return nil, fmt.Errorf("oauth: automatic refresh requires a saved connection with managed token storage")
	}
	if store == nil {
		store = oauth.NewMemoryStore(oauth.Token{RefreshToken: rendered["refresh_token"]})
	}
	source, err := oauth.NewSource(oauth.Config{
		TokenURL: rendered["token_url"], ClientID: rendered["client_id"], ClientSecret: rendered["client_secret"],
		GrantType: strParam(a.params, "grant_type"), ClientAuth: strParam(a.params, "client_auth"),
		Scope: rendered["scope"], Params: extra, HTTPClient: a.httpClient,
	}, store)
	if err != nil {
		return nil, err
	}
	a.source = source
	return source, nil
}

// RefreshRejected renews a rejected access token once through the shared store.
func RefreshRejected(ctx context.Context, a Authenticator, rejected string) (bool, error) {
	switch a := a.(type) {
	case *oauth2CC:
		a.mu.Lock()
		source := a.source
		a.mu.Unlock()
		if source == nil {
			return false, nil
		}
		_, err := source.Refresh(ctx, rejected)
		return true, err
	case *chainAuth:
		for _, child := range a.steps {
			if handled, err := RefreshRejected(ctx, child, rejected); handled || err != nil {
				return handled, err
			}
		}
	}
	return false, nil
}

// WithOAuthHTTPClient supplies the token endpoint transport before first use.
// Callers must not change it while requests are in flight.
func WithOAuthHTTPClient(a Authenticator, client *http.Client) {
	switch a := a.(type) {
	case *oauth2CC:
		a.httpClient = client
	case *chainAuth:
		for _, child := range a.steps {
			WithOAuthHTTPClient(child, client)
		}
	}
}
