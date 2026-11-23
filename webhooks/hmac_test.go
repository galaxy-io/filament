package webhooks

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // the GitHub legacy header signs with SHA1
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func sign(h func() hash.Hash, key, content []byte) []byte {
	mac := hmac.New(h, key)
	_, _ = mac.Write(content)
	return mac.Sum(nil)
}

func mustHMAC(t *testing.T, params string) Verifier {
	t.Helper()
	v, err := BuildVerifier(HMAC, json.RawMessage(params))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestHMACProviderSchemes(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"id":"evt_1","type":"push"}`)
	secret := []byte("s3cret")

	for name, tc := range map[string]struct {
		params string
		header http.Header
		id     string
	}{
		"github": {
			params: `{"signature":{"header":"X-Hub-Signature-256","prefix":"sha256="},"delivery_id":{"header":"X-GitHub-Delivery"}}`,
			header: http.Header{
				"X-Hub-Signature-256": {"sha256=" + hex.EncodeToString(sign(sha256.New, secret, body))},
				"X-Github-Delivery":   {"72d3162e-cc78-11e3-81ab-4c9367dc0958"},
			},
			id: "72d3162e-cc78-11e3-81ab-4c9367dc0958",
		},
		"github sha1": {
			params: `{"algorithm":"sha1","signature":{"header":"X-Hub-Signature","prefix":"sha1="},"delivery_id":{"header":"X-GitHub-Delivery"}}`,
			header: http.Header{
				"X-Hub-Signature":   {"sha1=" + hex.EncodeToString(sign(sha1.New, secret, body))},
				"X-Github-Delivery": {"d1"},
			},
			id: "d1",
		},
		"shopify": {
			params: `{"signature":{"header":"X-Shopify-Hmac-Sha256","encoding":"base64"},"delivery_id":{"header":"X-Shopify-Webhook-Id"}}`,
			header: http.Header{
				"X-Shopify-Hmac-Sha256": {base64.StdEncoding.EncodeToString(sign(sha256.New, secret, body))},
				"X-Shopify-Webhook-Id":  {"b54557e4-bdd9-4b37-8a5f-bf7d70bcd043"},
			},
			id: "b54557e4-bdd9-4b37-8a5f-bf7d70bcd043",
		},
		"stripe": {
			params: `{"signature":{"header":"Stripe-Signature","pair":"v1"},"timestamp":{"pair":"t"},"signed":"{timestamp}.{body}","delivery_id":{"body":"id"}}`,
			header: http.Header{"Stripe-Signature": {"t=" + ts + ",v1=" + hex.EncodeToString(sign(sha256.New, secret, []byte(ts+"."+string(body)))) + ",v0=ignored"}},
			id:     "evt_1",
		},
		"slack": {
			params: `{"signature":{"header":"X-Slack-Signature","prefix":"v0="},"timestamp":{"header":"X-Slack-Request-Timestamp"},"signed":"v0:{timestamp}:{body}","delivery_id":{"body":"id"}}`,
			header: http.Header{
				"X-Slack-Signature":         {"v0=" + hex.EncodeToString(sign(sha256.New, secret, []byte("v0:"+ts+":"+string(body))))},
				"X-Slack-Request-Timestamp": {ts},
			},
			id: "evt_1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := mustHMAC(t, tc.params)
			id, err := v.Verify(tc.header, body, [][]byte{[]byte("old"), secret}, now)
			if err != nil || id != tc.id {
				t.Fatalf("Verify = %q, %v; want %q", id, err, tc.id)
			}
			if _, err := v.Verify(tc.header, []byte(`{"id":"evt_1","type":"pull"}`), [][]byte{secret}, now); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("tampered body verified: %v", err)
			}
			if _, err := v.Verify(tc.header, body, [][]byte{[]byte("wrong")}, now); !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("wrong key verified: %v", err)
			}
		})
	}
}

// Metronome signs X-Metronome-Date, an HTTP-date, then a newline and the
// body. This is the example its documentation verifies.
// https://docs.metronome.com/guides/platform-configuration/webhooks/best-practices
func TestHMACMetronomeDocumentedExample(t *testing.T) {
	v := mustHMAC(t, `{"signature":{"header":"Metronome-Webhook-Signature"},"timestamp":{"header":"X-Metronome-Date","format":"http_date"},"signed":"{timestamp}\n{body}","delivery_id":{"body":"id"}}`)
	body := []byte(`{
  "id": "b2c9e307-624e-4e7d-a5a4-1b74107d78c4",
  "type": "widget_created",
  "properties": {
    "customer_id": "5f794d50-085a-4db6-8d15-286e518b7225",
    "widget_id": "0891458d-b6f0-4fdd-a41e-380aae1a1e38"
  }
}`)
	header := http.Header{
		"X-Metronome-Date":            {"Mon, 02 Jan 2006 22:04:05 GMT"},
		"Metronome-Webhook-Signature": {"b82652fa2246cf1d8a27e591f155c865f68b46c19b9213fd9c052f2419b4742b"},
	}
	sent := time.Date(2006, 1, 2, 22, 4, 5, 0, time.UTC)
	keys := [][]byte{[]byte("correct-horse-battery-staple")}
	if id, err := v.Verify(header, body, keys, sent.Add(time.Minute)); err != nil || id != "b2c9e307-624e-4e7d-a5a4-1b74107d78c4" {
		t.Fatalf("Verify = %q, %v", id, err)
	}
	if _, err := v.Verify(header, body, keys, sent.Add(6*time.Minute)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("stale delivery verified: %v", err)
	}
	header.Set("X-Metronome-Date", "1136239445")
	if _, err := v.Verify(header, body, keys, sent); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unix seconds accepted as an HTTP-date: %v", err)
	}
}

func TestHMACRejectsStaleTimestamp(t *testing.T) {
	v := mustHMAC(t, `{"signature":{"header":"X-Sig"},"timestamp":{"header":"X-Ts","tolerance_seconds":60},"signed":"{timestamp}.{body}","delivery_id":{"header":"X-Id"}}`)
	sent := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(sent.Unix(), 10)
	body := []byte("{}")
	header := http.Header{
		"X-Sig": {hex.EncodeToString(sign(sha256.New, []byte("k"), []byte(ts+".{}")))},
		"X-Ts":  {ts},
		"X-Id":  {"d1"},
	}
	if _, err := v.Verify(header, body, [][]byte{[]byte("k")}, sent.Add(59*time.Second)); err != nil {
		t.Fatalf("fresh delivery: %v", err)
	}
	if _, err := v.Verify(header, body, [][]byte{[]byte("k")}, sent.Add(61*time.Second)); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("stale delivery verified: %v", err)
	}
}

// The HMAC profile expresses Filament's own signing scheme, so a provider
// with the same shape needs no code.
func TestHMACMatchesGenericHMAC(t *testing.T) {
	v := mustHMAC(t, `{"signature":{"header":"X-Filament-Signature","prefix":"sha256="},"timestamp":{"header":"X-Filament-Timestamp"},"signed":"{timestamp}.{delivery_id}.{body}","delivery_id":{"header":"X-Filament-Delivery-Id"}}`)
	generic, _ := BuildVerifier(GenericHMAC, nil)
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"a":1}`)
	header := http.Header{
		"X-Filament-Signature":   {"sha256=" + hex.EncodeToString(sign(sha256.New, []byte("k"), []byte(ts+".d1."+string(body))))},
		"X-Filament-Timestamp":   {ts},
		"X-Filament-Delivery-Id": {"d1"},
	}
	for _, verifier := range []Verifier{v, generic} {
		if id, err := verifier.Verify(header, body, [][]byte{[]byte("k")}, now); err != nil || id != "d1" {
			t.Fatalf("Verify = %q, %v", id, err)
		}
	}
}

func TestHMACKeyFormat(t *testing.T) {
	v := mustHMAC(t, `{"signature":{"header":"X-Sig"},"delivery_id":{"header":"X-Id"},"key":{"prefix":"whsec_","encoding":"base64"}}`)
	format := v.(KeyFormat)
	key, err := format.MintKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := format.ValidateKey(key); err != nil {
		t.Fatalf("minted key %q does not validate: %v", key, err)
	}
	if err := format.ValidateKey("whsec_not base64!"); err == nil {
		t.Fatal("invalid key validated")
	}
	// The base64 decoder skips line breaks, so these decode to no bytes.
	for _, key := range []string{"whsec_\r\n", "whsec_"} {
		if err := format.ValidateKey(key); err == nil {
			t.Fatalf("key %q that decodes to nothing validated", key)
		}
	}
	unprefixed := mustHMAC(t, `{"signature":{"header":"X-Sig"},"delivery_id":{"header":"X-Id"},"key":{"encoding":"base64"}}`).(KeyFormat)
	if err := unprefixed.ValidateKey("\r\n"); err == nil {
		t.Fatal("unprefixed key that decodes to nothing validated")
	}
}

func TestHMACRejectsInvalidParams(t *testing.T) {
	for name, params := range map[string]string{
		"no header":                `{"delivery_id":{"header":"X-Id"}}`,
		"no delivery id":           `{"signature":{"header":"X-Sig"}}`,
		"unknown algorithm":        `{"algorithm":"md5","signature":{"header":"X-Sig"},"delivery_id":{"header":"X-Id"}}`,
		"unknown placeholder":      `{"signature":{"header":"X-Sig"},"signed":"{nope}.{body}","delivery_id":{"header":"X-Id"}}`,
		"no body signed":           `{"signature":{"header":"X-Sig"},"signed":"{delivery_id}","delivery_id":{"header":"X-Id"}}`,
		"timestamp unconfigured":   `{"signature":{"header":"X-Sig"},"signed":"{timestamp}.{body}","delivery_id":{"header":"X-Id"}}`,
		"pair without pairs":       `{"signature":{"header":"X-Sig"},"timestamp":{"pair":"t"},"delivery_id":{"header":"X-Id"}}`,
		"unknown field":            `{"signature":{"header":"X-Sig"},"delivery_id":{"header":"X-Id"},"extra":true}`,
		"unsigned timestamp":       `{"signature":{"header":"X-Sig"},"timestamp":{"header":"X-Ts"},"delivery_id":{"header":"X-Id"}}`,
		"unknown timestamp format": `{"signature":{"header":"X-Sig"},"timestamp":{"header":"X-Ts","format":"rfc3339"},"delivery_id":{"header":"X-Id"}}`,
		"query delivery id":        `{"signature":{"header":"X-Sig"},"delivery_id":{"query":"id"}}`,
	} {
		if _, err := BuildVerifier(HMAC, json.RawMessage(params)); err == nil {
			t.Errorf("%s: built", name)
		}
	}
}
