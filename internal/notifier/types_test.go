package notifier_test

import (
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

func TestNotificationTypeRoundTrip(t *testing.T) {
	label, err := notifier.NotificationTypeLabel(ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK)
	if err != nil || label != "webhook" {
		t.Fatalf("NotificationTypeLabel() = %q, %v", label, err)
	}
	parsed, err := notifier.ParseNotificationType(label)
	if err != nil || parsed != ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK {
		t.Fatalf("ParseNotificationType() = %d, %v", parsed, err)
	}
}

func TestNotificationTypeRejectsInvalidValues(t *testing.T) {
	for _, value := range []ingestionv1.NotificationType{
		ingestionv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED,
		-1,
		2,
		999,
	} {
		if _, err := notifier.NotificationTypeLabel(value); err == nil {
			t.Errorf("NotificationTypeLabel(%d) accepted invalid type", value)
		}
	}
	for _, label := range []string{"", "unspecified", "event", "WEBHOOK", " webhook", "1"} {
		if _, err := notifier.ParseNotificationType(label); err == nil {
			t.Errorf("ParseNotificationType(%q) accepted invalid label", label)
		}
	}
}
