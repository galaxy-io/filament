package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrReauthorizationRequired means the refresh grant is missing or no longer valid.
var ErrReauthorizationRequired = errors.New("oauth: authorization expired or revoked")

// EndpointError includes only an allowlisted OAuth code and HTTP status.
// Provider descriptions, response bodies, and credential-bearing URLs are
// deliberately excluded. Network errors preserve context cancellation only.
type EndpointError struct {
	StatusCode int
	Code       string
}

func (e *EndpointError) Error() string {
	return fmt.Sprintf("oauth: token endpoint HTTP %d (%s)", e.StatusCode, e.Code)
}

// Is identifies invalid_grant without exposing provider error descriptions.
func (e *EndpointError) Is(target error) bool {
	return target == ErrReauthorizationRequired && e.Code == "invalid_grant"
}

// Retryable reports an explicit temporary rejection. Network failures are
// ambiguous for rotating grants and must not be retried automatically.
func (e *EndpointError) Retryable() bool {
	if e.Code == "invalid_grant" || e.Code == "invalid_client" || e.Code == "unauthorized_client" || e.Code == "invalid_request" || e.Code == "unsupported_grant_type" || e.Code == "invalid_scope" {
		return false
	}
	return e.StatusCode == http.StatusTooManyRequests || e.Code == "temporarily_unavailable"
}

type tokenResponse struct {
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    json.RawMessage `json:"expires_in"`
	Error        string          `json:"error"`
}

func (s *Source) exchange(ctx context.Context, current Token) (Token, error) {
	started := s.now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.TokenURL, strings.NewReader(s.form(current).Encode()))
	if err != nil {
		return Token{}, errors.New("oauth: cannot construct token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if s.config.ClientAuth == ClientSecretBasic {
		// RFC 6749 section 2.3.1 encodes each component before HTTP Basic.
		req.SetBasicAuth(url.QueryEscape(s.config.ClientID), url.QueryEscape(s.config.ClientSecret))
	}
	resp, err := s.config.HTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Token{}, ctx.Err()
		}
		return Token{}, errors.New("oauth: token request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	const maxResponse = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return Token{}, errors.New("oauth: cannot read token response")
	}
	var response tokenResponse
	decodeErr := json.Unmarshal(body, &response)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || response.Error != "" {
		return Token{}, &EndpointError{StatusCode: resp.StatusCode, Code: safeCode(response.Error)}
	}
	if decodeErr != nil {
		return Token{}, errors.New("oauth: invalid token response")
	}
	if response.AccessToken == "" {
		return Token{}, errors.New("oauth: empty access token")
	}
	ttl, err := expiresIn(response.ExpiresIn)
	if err != nil {
		return Token{}, err
	}
	if response.RefreshToken == "" {
		response.RefreshToken = current.RefreshToken
	}
	if response.TokenType == "" {
		response.TokenType = "Bearer"
	}
	return Token{AccessToken: response.AccessToken, RefreshToken: response.RefreshToken, TokenType: response.TokenType, ExpiresAt: started.Add(ttl)}, nil
}

func expiresIn(raw json.RawMessage) (time.Duration, error) {
	// Some existing providers omit expires_in. Keep the
	// HTTP connector's bounded five-minute fallback for that case only.
	if len(raw) == 0 {
		return 5 * time.Minute, nil
	}
	value := string(raw)
	if value != "" && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, errors.New("oauth: invalid token expiry")
		}
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 || seconds > int64((1<<63-1)/time.Second) {
		return 0, errors.New("oauth: invalid token expiry")
	}
	return time.Duration(seconds) * time.Second, nil
}

func safeCode(code string) string {
	switch code {
	case "invalid_grant", "invalid_client", "invalid_request", "unauthorized_client", "unsupported_grant_type", "invalid_scope", "temporarily_unavailable", "server_error":
		return code
	default:
		return "token_request_failed"
	}
}
