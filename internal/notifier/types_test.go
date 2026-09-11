package notifier_test

import (
	"encoding/json"
	"testing"

	"github.com/galaxy-io/filament/internal/notifier"
)

func TestNotificationTypeRoundTrip(t *testing.T) {
	label, err := notifier.NotificationWebhook.Label()
	if err != nil || label != "webhook" {
		t.Fatalf("Label() = %q, %v", label, err)
	}
	parsed, err := notifier.ParseNotificationType(label)
	if err != nil || parsed != notifier.NotificationWebhook {
		t.Fatalf("ParseNotificationType() = %d, %v", parsed, err)
	}
	// Test the enum embedded in a frame, as it will be in attempt events.
	type frame struct {
		Type notifier.NotificationType `json:"notification_type"`
	}
	raw, err := json.Marshal(frame{Type: parsed})
	if err != nil || string(raw) != `{"notification_type":"webhook"}` {
		t.Fatalf("Marshal() = %s, %v", raw, err)
	}
	var decoded frame
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != parsed {
		t.Fatalf("round trip = %d, want %d", decoded.Type, parsed)
	}
}

func TestNotificationTypeRejectsInvalidValues(t *testing.T) {
	for _, value := range []notifier.NotificationType{notifier.NotificationUnspecified, -1, 2, 999} {
		if _, err := value.Label(); err == nil {
			t.Errorf("Label(%d) accepted invalid type", value)
		}
		if _, err := json.Marshal(value); err == nil {
			t.Errorf("Marshal(%d) accepted invalid type", value)
		}
	}
	for _, label := range []string{"", "unspecified", "event", "WEBHOOK", " webhook", "1"} {
		if _, err := notifier.ParseNotificationType(label); err == nil {
			t.Errorf("ParseNotificationType(%q) accepted invalid label", label)
		}
	}
}

func TestNotificationTypeRejectsInvalidJSONWithoutMutation(t *testing.T) {
	for _, raw := range []string{`null`, `0`, `1`, `true`, `[]`, `{}`, `""`, `"event"`, `"WEBHOOK"`, `"webhook`} {
		t.Run(raw, func(t *testing.T) {
			value := notifier.NotificationWebhook
			if err := json.Unmarshal([]byte(raw), &value); err == nil {
				t.Fatal("accepted invalid notification type")
			}
			if value != notifier.NotificationWebhook {
				t.Fatal("invalid input changed receiver")
			}
		})
	}
}
