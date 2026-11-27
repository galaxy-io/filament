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
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func generateTestP8Key(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), priv
}

func TestES256TokenGenerationAndCaching(t *testing.T) {
	pemKey, priv := generateTestP8Key(t)
	c, err := newClient("issuer-uuid-123", "KEYID12345", pemKey)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	now := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
	tok1, err := c.bearerToken(now)
	if err != nil {
		t.Fatal(err)
	}
	tok2, err := c.bearerToken(now.Add(5 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if tok1 != tok2 {
		t.Fatal("expected cached token within validity window")
	}
	parts := strings.Split(tok1, ".")
	if len(parts) != 3 {
		t.Fatalf("invalid JWT format: %s", tok1)
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var hdr map[string]string
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr["alg"] != "ES256" || hdr["kid"] != "KEYID12345" || hdr["typ"] != "JWT" {
		t.Fatalf("unexpected header: %+v", hdr)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sigBytes) != 64 {
		t.Fatalf("invalid ES256 signature length: %d", len(sigBytes))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])
	if !ecdsa.Verify(&priv.PublicKey, digest[:], r, s) {
		t.Fatal("ES256 signature verification failed")
	}
}

func TestClientRetriesRateLimitsAndServerErrors(t *testing.T) {
	pemKey, _ := generateTestP8Key(t)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "0")
		switch attempts {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			_, _ = w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer server.Close()
	c, err := newClient("issuer", "key", pemKey)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	c.baseURL, c.limiter = server.URL, rate.NewLimiter(rate.Inf, 1)
	var out jsonAPIPage
	if err := c.requestJSON(t.Context(), "apps", "/v1/apps", &out); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestClientDoesNotRetryAccessErrorsOrExposeSensitiveDetail(t *testing.T) {
	pemKey, _ := generateTestP8Key(t)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"id":"err-123","code":"FORBIDDEN_ERROR","title":"Access Denied","detail":"secret-token-value"}]}`))
	}))
	defer server.Close()
	c, err := newClient("issuer", "key", pemKey)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	c.baseURL = server.URL
	err = c.requestJSON(t.Context(), "apps", "/v1/apps", &jsonAPIPage{})
	var api *apiError
	if !errors.As(err, &api) || api.Status != http.StatusForbidden || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
	if strings.Contains(err.Error(), "secret-token-value") || !strings.Contains(err.Error(), "err-123") {
		t.Fatalf("unexpected error string: %v", err)
	}
}

func TestClientCancellationStopsRetryWait(t *testing.T) {
	pemKey, _ := generateTestP8Key(t)
	ctx, cancel := context.WithCancel(t.Context())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		cancel()
	}))
	defer server.Close()
	c, err := newClient("issuer", "key", pemKey)
	if err != nil {
		t.Fatal(err)
	}
	defer c.http.CloseIdleConnections()
	c.baseURL = server.URL
	if err := c.requestJSON(ctx, "apps", "/v1/apps", &jsonAPIPage{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
