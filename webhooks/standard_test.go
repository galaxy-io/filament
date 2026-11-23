package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// A verifier that takes no parameters accepts any empty JSON object.
func TestParameterlessVerifierAcceptsEmptyObjects(t *testing.T) {
	for _, params := range []string{"", "null", "{}", "{ }", "{\n}"} {
		if _, err := BuildVerifier(StandardWebhooks, json.RawMessage(params)); err != nil {
			t.Errorf("params %q: %v", params, err)
		}
	}
	if _, err := BuildVerifier(StandardWebhooks, json.RawMessage(`{ "a": 1 }`)); err == nil {
		t.Error("a non-empty object was accepted")
	}
}

func TestStandardWebhooks(t *testing.T) {
	v, err := BuildVerifier(StandardWebhooks, nil)
	if err != nil {
		t.Fatal(err)
	}
	format := v.(KeyFormat)
	minted, err := format.MintKey()
	if err != nil || format.ValidateKey(minted) != nil {
		t.Fatalf("minted key %q: %v", minted, err)
	}
	if format.ValidateKey("whsec_c2hvcnQ=") == nil || format.ValidateKey("not base64!") == nil {
		t.Fatal("short or malformed key accepted")
	}
	other, _ := format.MintKey()
	keys := [][]byte{[]byte(other), []byte(minted)}
	body := []byte(`{"event":{"uuid":"e-1"}}`)
	now := time.Unix(1_700_000_000, 0)
	sign := func(secret, id string, at time.Time, payload []byte) http.Header {
		key, err := standardKey([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		ts := strconv.FormatInt(at.Unix(), 10)
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(id + "." + ts + "." + string(payload)))
		h := http.Header{}
		h.Set("webhook-id", id)
		h.Set("webhook-timestamp", ts)
		h.Set("webhook-signature", "v1,AAAA v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		return h
	}
	id, err := v.Verify(sign(minted, "msg_1", now, body), body, keys, now)
	if err != nil || id != "msg_1" {
		t.Fatalf("id %q, err %v", id, err)
	}
	unknown, _ := format.MintKey()
	for name, h := range map[string]http.Header{
		"unknown key":   sign(unknown, "msg_1", now, body),
		"stale":         sign(minted, "msg_1", now.Add(-10*time.Minute), body),
		"tampered body": sign(minted, "msg_1", now, []byte(`{}`)),
		"no signature":  {"Webhook-Id": {"msg_1"}, "Webhook-Timestamp": {strconv.FormatInt(now.Unix(), 10)}},
	} {
		if _, err := v.Verify(h, body, keys, now); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: got %v, want ErrUnauthorized", name, err)
		}
	}
}
