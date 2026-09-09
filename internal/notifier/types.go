// Package notifier defines shared notification types and storage contracts.
package notifier

import (
	"encoding/json"
	"fmt"

	"github.com/galaxy-io/filament"
)

// NotificationType selects how a notification is sent.
type NotificationType int

const (
	// NotificationUnspecified means no type was selected.
	NotificationUnspecified NotificationType = 0
	// NotificationWebhook delivers an event to an HTTP endpoint.
	NotificationWebhook NotificationType = 1
)

// Label returns the database/JSON name, or an error for an invalid type.
func (t NotificationType) Label() (string, error) {
	switch t {
	case NotificationWebhook:
		return "webhook", nil
	default:
		return "", fmt.Errorf("invalid notification type %d", t)
	}
}

// ParseNotificationType reads a name such as "webhook", rejecting unknown names.
func ParseNotificationType(label string) (NotificationType, error) {
	switch label {
	case "webhook":
		return NotificationWebhook, nil
	default:
		return NotificationUnspecified, fmt.Errorf("invalid notification type %q", label)
	}
}

// MarshalJSON writes the type's name as a JSON string.
func (t NotificationType) MarshalJSON() ([]byte, error) {
	label, err := t.Label()
	if err != nil {
		return nil, err
	}
	return json.Marshal(label)
}

// UnmarshalJSON reads a type name. Invalid input leaves t unchanged.
func (t *NotificationType) UnmarshalJSON(data []byte) error {
	if t == nil {
		return fmt.Errorf("unmarshal notification type: nil receiver")
	}
	var label string
	if err := json.Unmarshal(data, &label); err != nil {
		return fmt.Errorf("unmarshal notification type: %w", err)
	}
	next, err := ParseNotificationType(label)
	if err != nil {
		return err
	}
	*t = next
	return nil
}

// Notifier defines when and how a pipeline sends notifications.
// It is stored separately from pipeline graph versions.
type Notifier struct {
	ID               string
	Tenant           filament.TenantID
	PipelineID       string
	Name             string
	NotificationType NotificationType
	Enabled          bool
	Events           []string          // Event names, or ["*"] for all eligible events.
	Resources        []string          // Empty matches any resource.
	Config           map[string]any    // Non-secret settings.
	SecretRefs       map[string]string // References to stored credentials.
	Version          int64             // Used to reject stale updates.
	// Times are Unix milliseconds; DeletedAt is zero until deletion.
	CreatedAt       int64
	UpdatedAt       int64
	DeletedAt       int64
	CreatedByUserID string
	UpdatedByUserID string
	DeletedByUserID string
}
