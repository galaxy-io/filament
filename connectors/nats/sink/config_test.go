package sink

import (
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestSharedStreamSubjectMustNotCaptureJetStreamAPI(t *testing.T) {
	for _, subject := range []string{"{resource}", "{{resource}}", "$JS.API.requests", "$JS.domain.API.requests"} {
		t.Run(subject, func(t *testing.T) {
			_, err := resolve(filament.NewConfig(map[string]any{"stream": "mystream", "subject": subject}))
			if err == nil || !strings.Contains(err.Error(), "literal prefix") {
				t.Fatalf("expected actionable subject error, got %v", err)
			}
		})
	}
	for _, subject := range []string{"filament.{resource}", "filament.{{resource}}", "events.orders"} {
		t.Run(subject, func(t *testing.T) {
			if _, err := resolve(filament.NewConfig(map[string]any{"stream": "mystream", "subject": subject})); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Per-resource streams capture concrete subjects, not a shared > wildcard.
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": "{resource}", "subject": "{resource}"})); err != nil {
		t.Fatal(err)
	}
	// Explicitly provisioned streams are validated against their actual config at open.
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": "mystream", "subject": "{resource}", "create_stream": false})); err != nil {
		t.Fatal(err)
	}
}
