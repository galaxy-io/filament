package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GenericHMAC is the verifier for Filament's own signing profile.
const GenericHMAC = "generic_hmac_sha256"

// ErrUnauthorized means the delivery's signature or identity did not verify.
var ErrUnauthorized = errors.New("webhook delivery is not authenticated")

// Verifier authenticates one provider's delivery format. It returns the
// provider's delivery ID, which deduplicates retries, or ErrUnauthorized.
// keys holds every currently valid signing key, primary first.
type Verifier interface {
	Verify(header http.Header, body []byte, keys [][]byte, now time.Time) (deliveryID string, err error)
}

// Profile builds a verifier from its parameters. A connector names a profile
// and supplies the parameters as data, so the receiver needs no connector
// code to authenticate a provider's deliveries. Empty params select the
// profile's defaults. A profile rejects parameters it does not understand.
type Profile func(params json.RawMessage) (Verifier, error)

var profiles sync.Map

// RegisterProfile makes a parameterized verifier selectable by name. It
// panics on a duplicate name or a nil profile, like connector registration.
func RegisterProfile(name string, p Profile) {
	if name == "" || p == nil {
		panic("webhooks: verifier name and implementation are required")
	}
	if _, loaded := profiles.LoadOrStore(name, p); loaded {
		panic("webhooks: duplicate verifier " + name)
	}
}

// RegisterVerifier makes a verifier that takes no parameters selectable by name.
func RegisterVerifier(name string, v Verifier) {
	if v == nil {
		panic("webhooks: verifier name and implementation are required")
	}
	RegisterProfile(name, func(params json.RawMessage) (Verifier, error) {
		if !emptyParams(params) {
			return nil, fmt.Errorf("verifier %q takes no parameters", name)
		}
		return v, nil
	})
}

// BuildVerifier returns the verifier name selects, configured by params.
func BuildVerifier(name string, params json.RawMessage) (Verifier, error) {
	p, ok := profiles.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown verifier %q", name)
	}
	return p.(Profile)(params)
}

func emptyParams(params json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(params))
	if trimmed == "" || trimmed == "null" {
		return true
	}
	var obj map[string]json.RawMessage
	return json.Unmarshal([]byte(trimmed), &obj) == nil && obj != nil && len(obj) == 0
}

// decodeParams strictly decodes a profile's parameters into dst.
func decodeParams(params json.RawMessage, dst any) error {
	if emptyParams(params) {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(params)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("parameters: %w", err)
	}
	return nil
}

func init() { RegisterVerifier(GenericHMAC, genericHMAC{}) }

const (
	maxDeliveryID = 256
	maxClockSkew  = 5 * time.Minute
)

// genericHMAC signs timestamp + "." + delivery ID + "." + raw body with
// HMAC-SHA256. X-Filament-Signature carries "sha256=" and lowercase hex.
type genericHMAC struct{}

func (genericHMAC) Verify(header http.Header, body []byte, keys [][]byte, now time.Time) (string, error) {
	ids := header.Values("X-Filament-Delivery-Id")
	times := header.Values("X-Filament-Timestamp")
	signatures := header.Values("X-Filament-Signature")
	if len(ids) != 1 || len(times) != 1 || len(signatures) != 1 || !validToken(ids[0], maxDeliveryID) {
		return "", ErrUnauthorized
	}
	hexSignature, ok := strings.CutPrefix(signatures[0], "sha256=")
	if !ok || len(hexSignature) != sha256.Size*2 {
		return "", ErrUnauthorized
	}
	signature, err := hex.DecodeString(hexSignature)
	if err != nil {
		return "", ErrUnauthorized
	}
	seconds, err := strconv.ParseInt(times[0], 10, 64)
	if err != nil {
		return "", ErrUnauthorized
	}
	if delta := now.Sub(time.Unix(seconds, 0)); delta < -maxClockSkew || delta > maxClockSkew {
		return "", ErrUnauthorized
	}
	signed := make([]byte, 0, len(times[0])+len(ids[0])+len(body)+2)
	signed = append(signed, times[0]...)
	signed = append(signed, '.')
	signed = append(signed, ids[0]...)
	signed = append(signed, '.')
	signed = append(signed, body...)
	valid := false
	for _, key := range keys {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write(signed)
		// Check every key so timing does not reveal which one matched.
		valid = hmac.Equal(mac.Sum(nil), signature) || valid
	}
	if !valid {
		return "", ErrUnauthorized
	}
	return ids[0], nil
}
