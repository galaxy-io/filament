package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"maps"
	"net/http"
	"strconv"
	"time"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("hmac", newHMAC) }

// hmacAuth signs requests by computing HMAC over a templated canonical string
// and writing it into a configurable header.
//
// The canonical string template can reference standard scopes plus the
// auto-injected `state.timestamp` (unix seconds at signing time).
//
// Params:
//
//	algorithm    — sha256 (default) | sha1 | sha512
//	encoding     — hex (default) | base64
//	secret       — secret key (template OK, e.g. "{{ config.signing_secret }}")
//	header_name  — destination header (e.g. "X-Signature")
//	scheme       — optional prefix (e.g. "HMAC")
//	canonical    — canonical string template
//	timestamp_header — optional header to also write the timestamp into
type hmacAuth struct {
	algorithm       string
	encoding        string
	secret          string
	headerName      string
	scheme          string
	canonical       string
	timestampHeader string
}

func newHMAC(params map[string]any) (Authenticator, error) {
	secret, err := requireStrParam(params, "hmac", "secret")
	if err != nil {
		return nil, err
	}
	header, err := requireStrParam(params, "hmac", "header_name")
	if err != nil {
		return nil, err
	}
	canon, err := requireStrParam(params, "hmac", "canonical")
	if err != nil {
		return nil, err
	}
	algo := strParam(params, "algorithm")
	if algo == "" {
		algo = "sha256"
	}
	enc := strParam(params, "encoding")
	if enc == "" {
		enc = "hex"
	}
	return &hmacAuth{
		algorithm:       algo,
		encoding:        enc,
		secret:          secret,
		headerName:      header,
		scheme:          strParam(params, "scheme"),
		canonical:       canon,
		timestampHeader: strParam(params, "timestamp_header"),
	}, nil
}

func (a *hmacAuth) Apply(_ context.Context, req *http.Request, scope template.Scope) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	// Copy-on-write: each Apply renders against its own snapshot of state
	// so concurrent Applies sharing a scope don't race on the timestamp
	// entry and the caller's State map is never mutated.
	state := make(map[string]string, len(scope.State)+1)
	maps.Copy(state, scope.State)
	state["timestamp"] = ts
	scope.State = state

	canon, err := template.Render(a.canonical, scope)
	if err != nil {
		return fmt.Errorf("hmac canonical: %w", err)
	}
	secret, err := template.Render(a.secret, scope)
	if err != nil {
		return fmt.Errorf("hmac secret: %w", err)
	}

	h, err := newHashFunc(a.algorithm)
	if err != nil {
		return err
	}
	mac := hmac.New(h, []byte(secret))
	mac.Write([]byte(canon))
	sum := mac.Sum(nil)

	var sig string
	switch a.encoding {
	case "hex":
		sig = hex.EncodeToString(sum)
	case "base64":
		sig = base64.StdEncoding.EncodeToString(sum)
	default:
		return fmt.Errorf("hmac: unknown encoding %q", a.encoding)
	}
	if a.scheme != "" {
		sig = a.scheme + " " + sig
	}
	req.Header.Set(a.headerName, sig)
	if a.timestampHeader != "" {
		req.Header.Set(a.timestampHeader, ts)
	}
	return nil
}

// DeclaredWrites reports the signature header and (when configured) the
// timestamp header.
func (a *hmacAuth) DeclaredWrites() []Write {
	out := []Write{{Kind: WriteHeader, Name: a.headerName}}
	if a.timestampHeader != "" {
		out = append(out, Write{Kind: WriteHeader, Name: a.timestampHeader})
	}
	return out
}

func newHashFunc(algo string) (func() hash.Hash, error) {
	switch algo {
	case "sha256":
		return sha256.New, nil
	case "sha1":
		return sha1.New, nil
	case "sha512":
		return sha512.New, nil
	}
	return nil, fmt.Errorf("hmac: unknown algorithm %q", algo)
}
