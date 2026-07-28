package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("oauth2_cc", newOAuth2CC) }

// oauth2CC implements the OAuth 2.0 Client Credentials grant.
//
// Cached tokens live in an atomic.Pointer so the hot path (token still fresh)
// is lock-free. Refreshes are coordinated by singleflight: any number of
// concurrent Applies whose cache check fails will collapse into exactly one
// upstream request to the token endpoint.
//
// retry is consulted on refresh failures (see RetryPolicy). Default is
// NoRetry (single-attempt).
type oauth2CC struct {
	tokenURL     string
	clientID     string
	clientSecret string
	scope        string

	httpClient *http.Client
	retry      RetryPolicy

	cache atomic.Pointer[cachedToken]
	sf    singleflight.Group
}

// WithOAuth2RetryPolicy attaches a RetryPolicy to an oauth2_cc Authenticator
// returned by Build("oauth2_cc", ...). Wraps the result so callers can opt
// into backoff behavior without restructuring registry construction.
//
// Calling on a non-oauth2_cc authenticator returns it unchanged.
func WithOAuth2RetryPolicy(a Authenticator, p RetryPolicy) Authenticator {
	if oc, ok := a.(*oauth2CC); ok && p != nil {
		oc.retry = p
	}
	return a
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

type oauth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func newOAuth2CC(params map[string]any) (Authenticator, error) {
	tokenURL, err := requireStrParam(params, "oauth2_cc", "token_url")
	if err != nil {
		return nil, err
	}
	clientID, err := requireStrParam(params, "oauth2_cc", "client_id")
	if err != nil {
		return nil, err
	}
	clientSecret, err := requireStrParam(params, "oauth2_cc", "client_secret")
	if err != nil {
		return nil, err
	}
	return &oauth2CC{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		scope:        strParam(params, "scope"),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		retry:        NoRetry{},
	}, nil
}

func (a *oauth2CC) Apply(ctx context.Context, req *http.Request, scope template.Scope) error {
	tok, err := a.token(ctx, scope)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

// DeclaredWrites reports that oauth2_cc writes the Authorization header.
func (a *oauth2CC) DeclaredWrites() []Write {
	return []Write{{Kind: WriteHeader, Name: "Authorization"}}
}

// token returns a still-fresh cached token without taking any lock; otherwise
// dispatches one upstream refresh via singleflight, with all concurrent
// callers receiving the same result. A 30s skew is reserved before the
// reported expiry so we don't race the issuer.
func (a *oauth2CC) token(ctx context.Context, scope template.Scope) (string, error) {
	if cur := a.cache.Load(); cur != nil && time.Now().Before(cur.expiresAt) {
		return cur.token, nil
	}
	v, err, _ := a.sf.Do("token", func() (any, error) {
		// Re-check after winning the singleflight slot: a previous winner
		// may have already populated the cache while we were queued.
		if cur := a.cache.Load(); cur != nil && time.Now().Before(cur.expiresAt) {
			return cur.token, nil
		}
		// Retry loop driven by RetryPolicy. attempt is 1-based and only
		// increments when the policy says retry.
		var lastErr error
		for attempt := 1; ; attempt++ {
			tok, fErr := a.fetch(ctx, scope)
			if fErr == nil {
				return tok, nil
			}
			lastErr = fErr
			delay, retry := a.retry.ShouldRetry(fErr, attempt)
			if !retry {
				return nil, lastErr
			}
			t := time.NewTimer(delay)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return nil, ctx.Err()
			}
		}
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// fetch performs the token endpoint POST and stores the result in the cache.
// Errors are wrapped with errs.ErrAuthRefresh so callers can distinguish
// refresh failures from other transport errors.
func (a *oauth2CC) fetch(ctx context.Context, scope template.Scope) (string, error) {
	tokenURL, err := template.Render(a.tokenURL, scope)
	if err != nil {
		return "", fmt.Errorf("%w: render token_url: %v", errs.ErrAuthRefresh, err)
	}
	clientID, err := template.Render(a.clientID, scope)
	if err != nil {
		return "", fmt.Errorf("%w: render client_id: %v", errs.ErrAuthRefresh, err)
	}
	clientSecret, err := template.Render(a.clientSecret, scope)
	if err != nil {
		return "", fmt.Errorf("%w: render client_secret: %v", errs.ErrAuthRefresh, err)
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	if a.scope != "" {
		form.Set("scope", a.scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: build token request: %v", errs.ErrAuthRefresh, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token request: %v", errs.ErrAuthRefresh, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%w: token endpoint HTTP %d: %s",
			errs.ErrAuthRefresh, resp.StatusCode, errs.FormatTruncatedN(string(body), 300))
	}

	var tr oauth2TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("%w: decode token response: %v", errs.ErrAuthRefresh, err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("%w: empty access_token in response", errs.ErrAuthRefresh)
	}

	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if ttl > 30*time.Second {
		ttl -= 30 * time.Second
	}
	a.cache.Store(&cachedToken{
		token:     tr.AccessToken,
		expiresAt: time.Now().Add(ttl),
	})
	return tr.AccessToken, nil
}
