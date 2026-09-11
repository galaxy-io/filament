// Package webhook defines webhook destination configuration and validation.
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"
)

// Destination is stored as one secret, including URL path and query credentials.
type Destination struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ParseDestination decodes and validates a resolved secret without exposing it in errors.
func ParseDestination(raw []byte) (Destination, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var destination Destination
	if err := decoder.Decode(&destination); err != nil {
		return Destination{}, fmt.Errorf("webhook: invalid destination JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Destination{}, fmt.Errorf("webhook: invalid destination JSON")
	}
	return NormalizeAndValidateDestination(destination)
}

// NormalizeAndValidateDestination checks settings and canonicalizes header names.
func NormalizeAndValidateDestination(d Destination) (Destination, error) {
	if err := validateURL(d.URL); err != nil {
		return Destination{}, err
	}
	headers := make(map[string]string, len(d.Headers))
	for name, value := range d.Headers {
		if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) || !utf8.ValidString(value) {
			return Destination{}, fmt.Errorf("webhook: invalid header name or value")
		}
		name = textproto.CanonicalMIMEHeaderKey(name)
		if name == "Content-Type" || strings.HasPrefix(name, "X-Filament-") {
			return Destination{}, fmt.Errorf("webhook: header is set by Filament")
		}
		if _, exists := headers[name]; exists {
			return Destination{}, fmt.Errorf("webhook: duplicate header name")
		}
		headers[name] = value
	}
	d.Headers = headers
	return d, nil
}

func validateURL(raw string) error {
	if !utf8.ValidString(raw) {
		return fmt.Errorf("webhook: URL must be valid text")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("webhook: URL must be absolute HTTP(S)")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("webhook: URL must use HTTP or HTTPS")
	}
	if u.User != nil || strings.Contains(raw, "#") {
		return fmt.Errorf("webhook: URL userinfo and fragments are not allowed")
	}
	return nil
}
