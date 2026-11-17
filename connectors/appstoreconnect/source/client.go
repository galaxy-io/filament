package appstoreconnect

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
)

const (
	defaultBaseURL = "https://api.appstoreconnect.apple.com"
	jwtTTL         = 15 * time.Minute
	jwtRefreshSkew = 2 * time.Minute
)

type client struct {
	http       *http.Client
	baseURL    string
	issuerID   string
	keyID      string
	privateKey *ecdsa.PrivateKey
	limiter    *rate.Limiter
	observe    filament.SourceObserver

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

func newClient(issuerID, keyID, rawPrivateKey string) (*client, error) {
	priv, err := parseECPrivateKey(rawPrivateKey)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 60 * time.Second
	return &client{
		http:       &http.Client{Transport: transport},
		baseURL:    defaultBaseURL,
		issuerID:   issuerID,
		keyID:      keyID,
		privateKey: priv,
		limiter:    rate.NewLimiter(2, 1),
	}, nil
}

func parseECPrivateKey(raw string) (*ecdsa.PrivateKey, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), `\n`, "\n")
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok || ecKey.Curve != elliptic.P256() {
			return nil, fmt.Errorf("PKCS#8 key must be ECDSA P-256")
		}
		return ecKey, nil
	}
	if ecKey, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		if ecKey.Curve != elliptic.P256() {
			return nil, fmt.Errorf("EC key must be P-256")
		}
		return ecKey, nil
	}
	return nil, fmt.Errorf("unsupported private key format (expected PKCS#8 .p8 P-256)")
}

func (c *client) bearerToken(now time.Time) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken != "" && now.Add(jwtRefreshSkew).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}
	exp := now.UTC().Add(jwtTTL)
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "ES256",
		"kid": c.keyID,
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(map[string]any{
		"iss": c.issuerID,
		"iat": now.UTC().Unix(),
		"exp": exp.Unix(),
		"aud": "appstoreconnect-v1",
	})
	if err != nil {
		return "", err
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	rInt, sInt, err := ecdsa.Sign(rand.Reader, c.privateKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign ES256 token: %w", err)
	}
	sig := make([]byte, 64)
	rInt.FillBytes(sig[:32])
	sInt.FillBytes(sig[32:])
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
	c.cachedToken = token
	c.tokenExpiry = exp
	return token, nil
}

type apiError struct {
	Status int    `json:"-"`
	ID     string `json:"id"`
	Code   string `json:"code"`
	Title  string `json:"title"`
}

func (e *apiError) Error() string {
	return fmt.Sprintf("App Store Connect HTTP %d (%s, id %s)", e.Status, e.Code, e.ID)
}

func parseAPIError(status int, data []byte) *apiError {
	failure := &apiError{Status: status}
	var envelope struct {
		Errors []struct {
			ID    string `json:"id"`
			Code  string `json:"code"`
			Title string `json:"title"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Errors) > 0 {
		failure.ID = envelope.Errors[0].ID
		failure.Code = envelope.Errors[0].Code
		failure.Title = envelope.Errors[0].Title
	}
	return failure
}

func (c *client) requestJSON(ctx context.Context, resource, method, path string, output any) error {
	data, err := c.requestBytes(ctx, resource, method, path, "application/json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *client) requestBytes(ctx context.Context, resource, method, pathOrURL, accept string) ([]byte, error) {
	targetURL := pathOrURL
	isExternal := strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://")
	if !isExternal {
		targetURL = c.baseURL + pathOrURL
	}
	for attempt := range 10 {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
		if err != nil {
			return nil, err
		}
		if !isExternal || sameOrigin(targetURL, c.baseURL) {
			tok, err := c.bearerToken(time.Now())
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return data, nil
		}
		failure := parseAPIError(resp.StatusCode, data)
		if attempt == 9 || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500) {
			return nil, failure
		}
		delay := retryDelay(resp.Header.Get("Retry-After"), attempt, time.Now())
		if c.observe != nil && resp.StatusCode == http.StatusTooManyRequests {
			c.observe(filament.SourceProgress{Kind: filament.SourceProgressRateLimited, Resource: resource, RetryAfter: delay})
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, nil
}

// sameOrigin reports whether rawURL uses the same scheme and host as base.
func sameOrigin(rawURL, base string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	return u.Scheme == b.Scheme && strings.EqualFold(u.Host, b.Host)
}

func retryDelay(value string, attempt int, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return time.Second << min(attempt, 5)
}
