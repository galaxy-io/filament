package slack

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	for _, raw := range []string{
		"https://hooks.slack.com/services/T000/B000/XXXX",
		"https://hooks.slack-gov.com/services/T000/B000/XXXX",
	} {
		if err := ValidateURL(raw); err != nil {
			t.Errorf("ValidateURL(%q) = %v", raw, err)
		}
	}
	for _, raw := range []string{
		"",
		"hooks.slack.com/services/T000/B000/XXXX",
		"http://hooks.slack.com/services/T000/B000/XXXX",
		"https://hooks.slack.com/services/",
		"https://hooks.slack.com/triggers/T000/B000/XXXX",
		"https://hooks.slack.com:8443/services/T000/B000/XXXX",
		"https://user@hooks.slack.com/services/T000/B000/XXXX",
		"https://hooks.slack.com/services/T000/B000/XXXX?x=1",
		"https://hooks.slack.com/services/T000/B000/XXXX#x",
		"https://hooks.slack.com.example.com/services/T000/B000/XXXX",
		"https://example.com/services/T000/B000/XXXX",
	} {
		if err := ValidateURL(raw); err == nil {
			t.Errorf("ValidateURL(%q) accepted an invalid URL", raw)
		}
	}
}

func TestDestinationErrorsOmitTheURL(t *testing.T) {
	const secret = "T000/B000/XXXX"
	_, err := DestinationFromConfig(map[string]any{URLField: "http://hooks.slack.com/services/" + secret})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("DestinationFromConfig() = %v", err)
	}
	_, err = ParseDestination([]byte(`{"url":"https://example.com/services/` + secret + `"}`))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("ParseDestination() = %v", err)
	}
}

func TestDestinationFromConfigTrimsWhitespace(t *testing.T) {
	d, err := DestinationFromConfig(map[string]any{URLField: " https://hooks.slack.com/services/T000/B000/XXXX\n"})
	if err != nil || d.URL != "https://hooks.slack.com/services/T000/B000/XXXX" {
		t.Fatalf("DestinationFromConfig() = %q, %v", d.URL, err)
	}
}
