// Package slack delivers notifications to Slack incoming webhooks.
package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/galaxy-io/filament"
)

// URLField is the config field and secret_refs key for the incoming webhook URL.
const URLField = "url"

// ConfigSchema declares the Slack fields: a secret incoming webhook URL.
var ConfigSchema = filament.ConfigSchema{Fields: []filament.ConfigField{
	{Name: URLField, Type: filament.FieldString, Required: true, Secret: true},
}}

// Destination is the resolved delivery target. The URL is a credential.
type Destination struct {
	URL string `json:"url"`
}

// ParseDestination decodes and validates a resolved destination without exposing it in errors.
func ParseDestination(raw []byte) (Destination, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var destination Destination
	if err := decoder.Decode(&destination); err != nil {
		return Destination{}, fmt.Errorf("slack: invalid destination JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Destination{}, fmt.Errorf("slack: invalid destination JSON")
	}
	if err := ValidateURL(destination.URL); err != nil {
		return Destination{}, err
	}
	return destination, nil
}

// DestinationFromConfig reads url from an effective config whose secret
// fields have been resolved.
func DestinationFromConfig(cfg map[string]any) (Destination, error) {
	raw, _ := cfg[URLField].(string)
	destination := Destination{URL: strings.TrimSpace(raw)}
	if err := ValidateURL(destination.URL); err != nil {
		return Destination{}, err
	}
	return destination, nil
}

// ValidateURL requires a Slack incoming webhook URL.
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return errURL
	}
	if u.Hostname() != "hooks.slack.com" && u.Hostname() != "hooks.slack-gov.com" {
		return errURL
	}
	if u.RawQuery != "" || strings.Contains(raw, "#") || len(u.Path) <= len(servicesPath) || !strings.HasPrefix(u.Path, servicesPath) {
		return errURL
	}
	return nil
}

const servicesPath = "/services/"

var errURL = fmt.Errorf("slack: URL must be a Slack incoming webhook URL")
