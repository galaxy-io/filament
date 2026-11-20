package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestSharedSecretHeaderRejectsInvalidParams(t *testing.T) {
	if _, err := BuildVerifier(SharedSecretHeader, json.RawMessage(`{"delivery_id":{"body":"id"},"extra":1}`)); err == nil {
		t.Fatal("shared secret accepted an unknown parameter")
	}
	if _, err := BuildVerifier(SharedSecretHeader, json.RawMessage(`{"delivery_id":{}}`)); err == nil {
		t.Fatal("shared secret accepted no delivery ID source")
	}
	if _, err := BuildVerifier(SharedSecretHeader, json.RawMessage(`{"delivery_id":{"header":"a","body":"b"}}`)); err == nil {
		t.Fatal("shared secret accepted two delivery ID sources")
	}
}

func TestSharedSecretHeader(t *testing.T) {
	v, err := BuildVerifier(SharedSecretHeader, json.RawMessage(
		`{"header":"x-filament-token","prefix":"Bearer ","delivery_id":{"body":"event.uuid"}}`))
	if err != nil {
		t.Fatal(err)
	}
	keys := [][]byte{[]byte("primary-secret-value"), []byte("previous-secret-value")}
	body := []byte(`{"event":{"uuid":"0190a7b2-6f1c-7c3e-9d0e-5a1b2c3d4e5f","event":"$pageview"}}`)
	header := func(values ...string) http.Header {
		h := http.Header{}
		for _, value := range values {
			h.Add("X-Filament-Token", value)
		}
		return h
	}
	now := time.Now()

	for _, key := range []string{"primary-secret-value", "previous-secret-value"} {
		id, err := v.Verify(header("Bearer "+key), body, keys, now)
		if err != nil || id != "0190a7b2-6f1c-7c3e-9d0e-5a1b2c3d4e5f" {
			t.Fatalf("key %q: id %q, err %v", key, id, err)
		}
	}
	for name, tc := range map[string]struct {
		header http.Header
		body   []byte
	}{
		"wrong secret":      {header("Bearer wrong-secret-value"), body},
		"missing prefix":    {header("primary-secret-value"), body},
		"no header":         {header(), body},
		"repeated header":   {header("Bearer primary-secret-value", "Bearer primary-secret-value"), body},
		"no delivery ID":    {header("Bearer primary-secret-value"), []byte(`{"event":{}}`)},
		"object ID":         {header("Bearer primary-secret-value"), []byte(`{"event":{"uuid":{}}}`)},
		"unsafe ID":         {header("Bearer primary-secret-value"), []byte(`{"event":{"uuid":"a/b"}}`)},
		"invalid JSON body": {header("Bearer primary-secret-value"), []byte(`{"event":`)},
	} {
		if _, err := v.Verify(tc.header, tc.body, keys, now); !errors.Is(err, ErrUnauthorized) {
			t.Errorf("%s: got %v, want ErrUnauthorized", name, err)
		}
	}
}

func TestSharedSecretHeaderDeliveryIDFromHeader(t *testing.T) {
	v, err := BuildVerifier(SharedSecretHeader, json.RawMessage(`{"delivery_id":{"header":"X-Delivery"}}`))
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{}
	h.Set("Authorization", "s3cret-s3cret-s3cret")
	h.Set("X-Delivery", "d-1")
	id, err := v.Verify(h, []byte("not json"), [][]byte{[]byte("s3cret-s3cret-s3cret")}, time.Now())
	if err != nil || id != "d-1" {
		t.Fatalf("id %q, err %v", id, err)
	}
}
