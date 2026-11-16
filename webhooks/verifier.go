package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

var verifiers sync.Map

// RegisterVerifier makes a verifier selectable by name. It panics on a
// duplicate name or a nil verifier, like connector registration.
func RegisterVerifier(name string, v Verifier) {
	if name == "" || v == nil {
		panic("webhooks: verifier name and implementation are required")
	}
	if _, loaded := verifiers.LoadOrStore(name, v); loaded {
		panic("webhooks: duplicate verifier " + name)
	}
}

// LookupVerifier returns the verifier registered under name.
func LookupVerifier(name string) (Verifier, bool) {
	v, ok := verifiers.Load(name)
	if !ok {
		return nil, false
	}
	return v.(Verifier), true
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
