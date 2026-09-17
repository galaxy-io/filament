// Package notifier contains private notification validation and delivery contracts.
package notifier

import (
	"fmt"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// NotificationTypeLabel returns the database and wire name for a notification type.
func NotificationTypeLabel(t ingestionv1.NotificationType) (string, error) {
	switch t {
	case ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK:
		return "webhook", nil
	default:
		return "", fmt.Errorf("invalid notification type %d", t)
	}
}

// ParseNotificationType reads a database name such as "webhook".
func ParseNotificationType(label string) (ingestionv1.NotificationType, error) {
	switch label {
	case "webhook":
		return ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK, nil
	default:
		return ingestionv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED, fmt.Errorf("invalid notification type %q", label)
	}
}
