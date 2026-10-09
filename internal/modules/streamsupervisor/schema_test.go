package streamsupervisor

import (
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

func TestSchemaBlockSurvivesRestartUntilIntentChanges(t *testing.T) {
	now := time.Now()
	state := filament.StreamState{Revision: 3, Attempt: &filament.Attempt{Revision: 3, EndedAt: &now, Reason: "schema_blocked: users: type changed"}}
	if !schemaBlocked(state) {
		t.Fatal("persisted schema block retried")
	}
	state.Revision++
	if schemaBlocked(state) {
		t.Fatal("manual intent change cannot retry")
	}
	state.Revision--
	state.Attempt.Reason = "network unavailable"
	if schemaBlocked(state) {
		t.Fatal("transient failure treated as schema block")
	}
	state.Attempt.Reason = "schema_blocked: users"
	state.Attempt.EndedAt = nil
	if schemaBlocked(state) {
		t.Fatal("active attempt treated as ended")
	}
}
