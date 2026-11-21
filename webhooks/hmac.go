package webhooks

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // some providers still sign with HMAC-SHA1
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net/http"
	"net/textproto"
	"slices"
	"strconv"
	"strings"
	"time"
)

// HMAC is a parameterized HMAC verifier for providers that sign a request
// body, optionally with a timestamp and delivery ID, and send the signature
// in a header. It covers schemes such as GitHub's X-Hub-Signature-256,
// Shopify's X-Shopify-Hmac-Sha256, Slack's v0 signature and Stripe's
// Stripe-Signature as data, so a connector needs no code to verify them.
const HMAC = "hmac"

// HMACParams configures the HMAC profile.
type HMACParams struct {
	// Algorithm is sha256 (the default), sha1 or sha512.
	Algorithm string `json:"algorithm,omitempty"`
	// Signature is where the signature is and how it is encoded.
	Signature HMACSignature `json:"signature"`
	// Timestamp, when set, is signed and checked against Tolerance.
	Timestamp *HMACTimestamp `json:"timestamp,omitempty"`
	// Signed is the signed content: a template over {body}, {timestamp} and
	// {delivery_id}. It defaults to {body}.
	Signed string `json:"signed,omitempty"`
	// DeliveryID names the provider's delivery ID, which deduplicates retries.
	DeliveryID ValueSource `json:"delivery_id"`
	// Key is how a stored secret becomes the HMAC key.
	Key HMACKey `json:"key,omitempty"`
}

// HMACSignature locates the signature. A plain header holds one or more
// space separated signatures. With Pair set, the header is a list of
// key=value entries, such as Stripe's "t=1700000000,v1=abc", and every entry
// named Pair is a candidate signature.
type HMACSignature struct {
	Header    string `json:"header"`
	Pair      string `json:"pair,omitempty"`
	Separator string `json:"separator,omitempty"`
	// Prefix precedes each encoded signature, such as "sha256=" or "v0=".
	Prefix string `json:"prefix,omitempty"`
	// Encoding is hex (the default) or base64.
	Encoding string `json:"encoding,omitempty"`
}

// HMACTimestamp locates the signing time in its own header or as an entry
// of the signature header's pairs. Format is unix, seconds since the epoch
// (the default), or http_date, an RFC 9110 HTTP-date such as
// "Mon, 02 Jan 2006 22:04:05 GMT". The signed content uses the value as sent.
type HMACTimestamp struct {
	Header string `json:"header,omitempty"`
	Pair   string `json:"pair,omitempty"`
	Format string `json:"format,omitempty"`
	// ToleranceSeconds bounds clock skew and replay. It defaults to 300.
	ToleranceSeconds int `json:"tolerance_seconds,omitempty"`
}

// HMACKey decodes a stored secret: Prefix, such as "whsec_", is removed and
// the rest decoded by Encoding, raw (the default), base64 or hex.
type HMACKey struct {
	Prefix   string `json:"prefix,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

const (
	timestampUnix     = "unix"
	timestampHTTPDate = "http_date"

	encodingRaw    = "raw"
	encodingHex    = "hex"
	encodingBase64 = "base64"

	placeholderBody       = "{body}"
	placeholderTimestamp  = "{timestamp}"
	placeholderDeliveryID = "{delivery_id}"
)

func init() { RegisterProfile(HMAC, newHMAC) }

type hmacVerifier struct {
	newHash   func() hash.Hash
	size      int
	signature HMACSignature
	timestamp *HMACTimestamp
	tolerance time.Duration
	signed    []string
	id        ValueSource
	key       HMACKey
}

var _ KeyFormat = hmacVerifier{}

func newHMAC(params json.RawMessage) (Verifier, error) {
	var p HMACParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	v := hmacVerifier{signature: p.Signature, timestamp: p.Timestamp, id: p.DeliveryID.canonical(), key: p.Key}
	var err error
	if v.newHash, v.size, err = hmacHash(p.Algorithm); err != nil {
		return nil, err
	}
	if err := v.normalizeSignature(); err != nil {
		return nil, err
	}
	if err := v.normalizeKey(); err != nil {
		return nil, err
	}
	if err := v.id.validate("hmac: delivery_id"); err != nil {
		return nil, err
	}
	signed := p.Signed
	if signed == "" {
		signed = placeholderBody
	}
	if v.signed, err = parseSigned(signed); err != nil {
		return nil, err
	}
	if err := v.normalizeTimestamp(strings.Contains(signed, placeholderTimestamp)); err != nil {
		return nil, err
	}
	return v, nil
}

func hmacHash(algorithm string) (func() hash.Hash, int, error) {
	switch algorithm {
	case "", "sha256":
		return sha256.New, sha256.Size, nil
	case "sha1":
		return sha1.New, sha1.Size, nil
	case "sha512":
		return sha512.New, sha512.Size, nil
	}
	return nil, 0, fmt.Errorf("hmac: unknown algorithm %q", algorithm)
}

func (v *hmacVerifier) normalizeSignature() error {
	if v.signature.Header == "" {
		return errors.New("hmac: signature.header is required")
	}
	v.signature.Header = textproto.CanonicalMIMEHeaderKey(v.signature.Header)
	if v.signature.Separator == "" {
		v.signature.Separator = ","
	}
	if v.signature.Separator != "," && v.signature.Pair == "" {
		return errors.New("hmac: signature.separator applies only with signature.pair")
	}
	switch v.signature.Encoding {
	case "":
		v.signature.Encoding = encodingHex
	case encodingHex, encodingBase64:
	default:
		return fmt.Errorf("hmac: unknown signature encoding %q", v.signature.Encoding)
	}
	return nil
}

func (v *hmacVerifier) normalizeKey() error {
	switch v.key.Encoding {
	case "":
		v.key.Encoding = encodingRaw
	case encodingRaw, encodingHex, encodingBase64:
	default:
		return fmt.Errorf("hmac: unknown key encoding %q", v.key.Encoding)
	}
	return nil
}

// normalizeTimestamp checks the timestamp settings and sets the tolerance.
// signsTimestamp reports whether the signed content uses {timestamp}.
func (v *hmacVerifier) normalizeTimestamp(signsTimestamp bool) error {
	ts := v.timestamp
	if ts == nil {
		if signsTimestamp {
			return errors.New("hmac: signed uses {timestamp} but no timestamp is configured")
		}
		return nil
	}
	if !signsTimestamp {
		// An unsigned timestamp can be replaced on a replayed request, so
		// checking its age would prove nothing.
		return errors.New("hmac: timestamp is configured but signed does not include {timestamp}")
	}
	if (ts.Header == "") == (ts.Pair == "") {
		return errors.New("hmac: timestamp needs exactly one of header or pair")
	}
	if ts.Pair != "" && v.signature.Pair == "" {
		return errors.New("hmac: timestamp.pair needs signature.pair")
	}
	if ts.Header != "" {
		ts.Header = textproto.CanonicalMIMEHeaderKey(ts.Header)
	}
	switch ts.Format {
	case "":
		ts.Format = timestampUnix
	case timestampUnix, timestampHTTPDate:
	default:
		return fmt.Errorf("hmac: unknown timestamp format %q", ts.Format)
	}
	if ts.ToleranceSeconds < 0 {
		return errors.New("hmac: timestamp.tolerance_seconds must not be negative")
	}
	v.tolerance = time.Duration(ts.ToleranceSeconds) * time.Second
	if v.tolerance == 0 {
		v.tolerance = maxClockSkew
	}
	return nil
}

// parseSigned splits the signed template into literals and placeholders.
func parseSigned(template string) ([]string, error) {
	var parts []string
	rest := template
	for rest != "" {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			parts = append(parts, rest)
			break
		}
		if start > 0 {
			parts = append(parts, rest[:start])
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			return nil, fmt.Errorf("hmac: unterminated placeholder in signed %q", template)
		}
		placeholder := rest[start : start+end+1]
		switch placeholder {
		case placeholderBody, placeholderTimestamp, placeholderDeliveryID:
			parts = append(parts, placeholder)
		default:
			return nil, fmt.Errorf("hmac: unknown placeholder %s in signed", placeholder)
		}
		rest = rest[start+end+1:]
	}
	if !slices.Contains(parts, placeholderBody) {
		return nil, errors.New("hmac: signed must include {body}")
	}
	return parts, nil
}

func (v hmacVerifier) Verify(header http.Header, body []byte, keys [][]byte, now time.Time) (string, error) {
	id, err := deliveryID(v.id, header, body)
	if err != nil {
		return "", err
	}
	if err := v.check(header, body, id, keys, now); err != nil {
		return "", err
	}
	return id, nil
}

func (v hmacVerifier) check(header http.Header, body []byte, id string, keys [][]byte, now time.Time) error {
	values := header.Values(v.signature.Header)
	if len(values) != 1 {
		return ErrUnauthorized
	}
	encoded, pairs := v.candidates(values[0])
	var timestamp string
	if v.timestamp != nil {
		var ok bool
		if timestamp, ok = v.signingTime(header, pairs); !ok {
			return ErrUnauthorized
		}
		signedAt, err := v.parseTime(timestamp)
		if err != nil {
			return ErrUnauthorized
		}
		if delta := now.Sub(signedAt); delta < -v.tolerance || delta > v.tolerance {
			return ErrUnauthorized
		}
	}
	var presented [][]byte
	for _, entry := range encoded {
		entry, ok := strings.CutPrefix(entry, v.signature.Prefix)
		if !ok {
			continue
		}
		if sig, err := decode(v.signature.Encoding, entry); err == nil && len(sig) == v.size {
			presented = append(presented, sig)
		}
	}
	if len(presented) == 0 {
		return ErrUnauthorized
	}
	signed := v.content(body, timestamp, id)
	valid := false
	for _, secret := range keys {
		key, err := v.hmacKey(secret)
		if err != nil {
			continue
		}
		mac := hmac.New(v.newHash, key)
		_, _ = mac.Write(signed)
		expected := mac.Sum(nil)
		// Check every key and signature so timing does not reveal a match.
		for _, sig := range presented {
			valid = hmac.Equal(expected, sig) || valid
		}
	}
	if !valid {
		return ErrUnauthorized
	}
	return nil
}

// candidates returns the encoded signatures in the header value, and its
// key=value pairs when the signature is one of them.
func (v hmacVerifier) candidates(value string) ([]string, map[string][]string) {
	if v.signature.Pair == "" {
		return strings.Fields(value), nil
	}
	pairs := map[string][]string{}
	for _, entry := range strings.Split(value, v.signature.Separator) {
		key, val, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if ok {
			pairs[key] = append(pairs[key], val)
		}
	}
	return pairs[v.signature.Pair], pairs
}

func (v hmacVerifier) signingTime(header http.Header, pairs map[string][]string) (string, bool) {
	var values []string
	if v.timestamp.Header != "" {
		values = header.Values(v.timestamp.Header)
	} else {
		values = pairs[v.timestamp.Pair]
	}
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func (v hmacVerifier) parseTime(value string) (time.Time, error) {
	if v.timestamp.Format == timestampHTTPDate {
		return http.ParseTime(value)
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(seconds, 0), nil
}

func (v hmacVerifier) content(body []byte, timestamp, id string) []byte {
	size := 0
	for _, part := range v.signed {
		switch part {
		case placeholderBody:
			size += len(body)
		case placeholderTimestamp:
			size += len(timestamp)
		case placeholderDeliveryID:
			size += len(id)
		default:
			size += len(part)
		}
	}
	out := make([]byte, 0, size)
	for _, part := range v.signed {
		switch part {
		case placeholderBody:
			out = append(out, body...)
		case placeholderTimestamp:
			out = append(out, timestamp...)
		case placeholderDeliveryID:
			out = append(out, id...)
		default:
			out = append(out, part...)
		}
	}
	return out
}

func (v hmacVerifier) hmacKey(secret []byte) ([]byte, error) {
	trimmed := strings.TrimPrefix(string(secret), v.key.Prefix)
	if trimmed == "" {
		return nil, errors.New("hmac: the signing secret is empty")
	}
	if v.key.Encoding == encodingRaw {
		return []byte(trimmed), nil
	}
	key, err := decode(v.key.Encoding, trimmed)
	if err != nil {
		return nil, fmt.Errorf("hmac: the signing secret must be %s", v.key.Encoding)
	}
	// The base64 decoder skips line breaks, so a secret of only "\r\n"
	// decodes to nothing without an error.
	if len(key) == 0 {
		return nil, errors.New("hmac: the signing secret decodes to an empty key")
	}
	return key, nil
}

// MintKey returns a random secret in the form the key settings decode.
func (v hmacVerifier) MintKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	switch v.key.Encoding {
	case encodingHex:
		return v.key.Prefix + hex.EncodeToString(raw), nil
	case encodingBase64:
		return v.key.Prefix + base64.StdEncoding.EncodeToString(raw), nil
	default:
		return v.key.Prefix + base64.RawURLEncoding.EncodeToString(raw), nil
	}
}

// ValidateKey reports why an imported secret cannot verify deliveries.
func (v hmacVerifier) ValidateKey(key string) error {
	_, err := v.hmacKey([]byte(key))
	return err
}

func decode(encoding, value string) ([]byte, error) {
	if encoding == encodingBase64 {
		return base64.StdEncoding.DecodeString(value)
	}
	return hex.DecodeString(value)
}
