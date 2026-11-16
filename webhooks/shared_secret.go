package webhooks

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// SharedSecretHeader authenticates a delivery by a secret the sender repeats
// in a request header, for providers that let a user set headers but do not
// sign their requests. The secret proves the sender, not the body, and does
// not expire, so a captured request can be replayed. The delivery ID
// deduplicates a replay of an accepted delivery.
const SharedSecretHeader = "shared_secret_header"

// SharedSecretParams configures SharedSecretHeader. Header defaults to
// Authorization. Prefix, such as "Bearer ", precedes the secret in the value.
// DeliveryID names a header or body value.
type SharedSecretParams struct {
	Header     string      `json:"header,omitempty"`
	Prefix     string      `json:"prefix,omitempty"`
	DeliveryID ValueSource `json:"delivery_id"`
}

func init() { RegisterProfile(SharedSecretHeader, newSharedSecretHeader) }

type sharedSecretHeader struct {
	header, prefix string
	id             ValueSource
}

func newSharedSecretHeader(params json.RawMessage) (Verifier, error) {
	var p SharedSecretParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	if p.Header == "" {
		p.Header = "Authorization"
	}
	if err := p.DeliveryID.validate("delivery_id"); err != nil {
		return nil, err
	}
	return sharedSecretHeader{
		header: textproto.CanonicalMIMEHeaderKey(p.Header), prefix: p.Prefix, id: p.DeliveryID.canonical(),
	}, nil
}

func (v sharedSecretHeader) Verify(header http.Header, body []byte, keys [][]byte, _ time.Time) (string, error) {
	values := header.Values(v.header)
	if len(values) != 1 {
		return "", ErrUnauthorized
	}
	presented, ok := strings.CutPrefix(values[0], v.prefix)
	if !ok || presented == "" {
		return "", ErrUnauthorized
	}
	valid := 0
	for _, key := range keys {
		// Compare against every key so timing does not reveal which one matched.
		valid |= subtle.ConstantTimeCompare([]byte(presented), key)
	}
	if valid != 1 {
		return "", ErrUnauthorized
	}
	// The body is only read once the sender is authenticated.
	return deliveryID(v.id, header, body)
}
