package runner

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/schema"
)

func TestContinuousSchemaReasonSurvivesWrapping(t *testing.T) {
	block := &schema.BlockError{Resource: "items", Reasons: []string{"type changed"}}
	err := errors.Join(errors.New("pipeline stopped"), fmt.Errorf("ensure schema: %w", block))
	if reason := continuousReason(err); !strings.HasPrefix(reason, "schema_blocked:") || !strings.Contains(reason, "items") {
		t.Fatalf("block identity lost before persistence: %s", reason)
	}
	if continuousReason(nil) != "" || continuousReason(errors.New("network error")) != "network error" {
		t.Fatal("ordinary reasons changed")
	}
}
