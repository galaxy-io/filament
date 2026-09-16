// Package webhook defines webhook destination configuration and validation.
package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/textproto"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"

	"github.com/galaxy-io/filament"
)

// HeadersField is the config field and secret_refs key for request headers.
const HeadersField = "headers"

// ConfigSchema declares the webhook fields: a plain url and secret headers.
var ConfigSchema = filament.ConfigSchema{Fields: []filament.ConfigField{
	{Name: "url", Type: filament.FieldString, Required: true},
	{Name: HeadersField, Type: filament.FieldObject, Secret: true},
}}

// Destination is the resolved delivery target, including secret header values.
type Destination struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ParseDestination decodes and validates a resolved destination without exposing it in errors.
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

// DestinationFromConfig reads url and optional headers from an effective
// config whose secret fields have been resolved.
func DestinationFromConfig(cfg map[string]any) (Destination, error) {
	rawURL, _ := cfg["url"].(string)
	destination := Destination{URL: rawURL}
	if raw, present := cfg[HeadersField]; present && raw != nil {
		object, ok := raw.(map[string]any)
		if !ok {
			return Destination{}, fmt.Errorf("webhook: headers must be an object")
		}
		destination.Headers = make(map[string]string, len(object))
		for name, value := range object {
			s, ok := value.(string)
			if !ok {
				return Destination{}, fmt.Errorf("webhook: header values must be strings")
			}
			destination.Headers[name] = s
		}
	}
	return NormalizeAndValidateDestination(destination)
}

// NormalizeAndValidateDestination checks settings and canonicalizes header names.
func NormalizeAndValidateDestination(d Destination) (Destination, error) {
	if err := ValidateURL(d.URL); err != nil {
		return Destination{}, err
	}
	headers, err := NormalizeAndValidateHeaders(d.Headers)
	if err != nil {
		return Destination{}, err
	}
	d.Headers = headers
	return d, nil
}

// NormalizeAndValidateHeaders canonicalizes header names and rejects names
// Filament sets, invalid characters, and duplicates.
func NormalizeAndValidateHeaders(in map[string]string) (map[string]string, error) {
	headers := make(map[string]string, len(in))
	for name, value := range in {
		if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) || !utf8.ValidString(value) {
			return nil, fmt.Errorf("webhook: invalid header name or value")
		}
		name = textproto.CanonicalMIMEHeaderKey(name)
		if name == "Host" || name == "Content-Type" || strings.HasPrefix(name, "X-Filament-") {
			return nil, fmt.Errorf("webhook: header is set by Filament")
		}
		if _, exists := headers[name]; exists {
			return nil, fmt.Errorf("webhook: duplicate header name")
		}
		headers[name] = value
	}
	return headers, nil
}

// ValidateURL requires an absolute public HTTP(S) URL without userinfo or a fragment.
func ValidateURL(raw string) error {
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
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicAddr(ip) {
		return errPrivateAddress
	}
	return nil
}

// errPrivateAddress marks a destination inside a private or local range.
var errPrivateAddress = errors.New("webhook: destination resolves to a private or local address")

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// publicAddr reports whether ip is routable beyond private and local ranges.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() && !cgnat.Contains(ip)
}
