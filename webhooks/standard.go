package webhooks

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// StandardWebhooks verifies the Standard Webhooks signature scheme
// (https://www.standardwebhooks.com): HMAC-SHA256 over
// "<webhook-id>.<webhook-timestamp>.<body>", sent as one or more space
// separated "v1,<base64>" entries in webhook-signature. The key is the
// base64 secret, with or without a "whsec_" prefix. The webhook-id is the
// delivery ID, which the spec keeps constant across retries.
const StandardWebhooks = "standard_webhooks"

// minStandardKeyBytes is the spec's floor for symmetric signing secrets.
const minStandardKeyBytes = 24

// KeyFormat is implemented by a verifier whose provider requires signing
// keys in a particular form. The server mints and checks keys with it.
type KeyFormat interface {
	// MintKey returns a new random key in the form the sender accepts.
	MintKey() (string, error)
	// ValidateKey reports why an imported key cannot verify deliveries.
	ValidateKey(key string) error
}

func init() { RegisterVerifier(StandardWebhooks, standardWebhooks{}) }

type standardWebhooks struct{}

var _ KeyFormat = standardWebhooks{}

func (standardWebhooks) MintKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(raw), nil
}

func (standardWebhooks) ValidateKey(key string) error {
	_, err := standardKey([]byte(key))
	return err
}

// standardKey decodes a Standard Webhooks secret into its HMAC key.
func standardKey(secret []byte) ([]byte, error) {
	encoded := strings.TrimPrefix(string(secret), "whsec_")
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("a Standard Webhooks secret must be base64, optionally prefixed with whsec_")
	}
	if len(key) < minStandardKeyBytes {
		return nil, fmt.Errorf("a Standard Webhooks secret must decode to at least %d bytes", minStandardKeyBytes)
	}
	return key, nil
}

func (standardWebhooks) Verify(header http.Header, body []byte, keys [][]byte, now time.Time) (string, error) {
	ids := header.Values("Webhook-Id")
	times := header.Values("Webhook-Timestamp")
	signatures := header.Values("Webhook-Signature")
	if len(ids) != 1 || len(times) != 1 || len(signatures) != 1 || !validToken(ids[0], maxDeliveryID) {
		return "", ErrUnauthorized
	}
	seconds, err := strconv.ParseInt(times[0], 10, 64)
	if err != nil {
		return "", ErrUnauthorized
	}
	if delta := now.Sub(time.Unix(seconds, 0)); delta < -maxClockSkew || delta > maxClockSkew {
		return "", ErrUnauthorized
	}
	var presented [][]byte
	for _, entry := range strings.Fields(signatures[0]) {
		encoded, ok := strings.CutPrefix(entry, "v1,")
		if !ok {
			continue
		}
		if sig, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(sig) == sha256.Size {
			presented = append(presented, sig)
		}
	}
	if len(presented) == 0 {
		return "", ErrUnauthorized
	}
	signed := make([]byte, 0, len(ids[0])+len(times[0])+len(body)+2)
	signed = append(signed, ids[0]...)
	signed = append(signed, '.')
	signed = append(signed, times[0]...)
	signed = append(signed, '.')
	signed = append(signed, body...)
	valid := false
	for _, secret := range keys {
		key, err := standardKey(secret)
		if err != nil {
			continue
		}
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write(signed)
		expected := mac.Sum(nil)
		// Check every key and signature so timing does not reveal a match.
		for _, sig := range presented {
			valid = hmac.Equal(expected, sig) || valid
		}
	}
	if !valid {
		return "", ErrUnauthorized
	}
	return ids[0], nil
}
