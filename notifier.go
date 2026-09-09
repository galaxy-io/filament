package filament

import (
	"context"
	"encoding/json"
	"fmt"
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
	Tenant           TenantID
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

// NotifierFilter selects a pipeline's rules, including disabled ones.
type NotifierFilter struct {
	Tenant         TenantID
	PipelineID     string
	IncludeDeleted bool
}

// NotifierStore is optional storage for pipeline notification rules.
// Each write transaction must check that the pipeline still exists and is not deleted.
// Updates and deletes require a matching version or return ErrVersionConflict.
type NotifierStore interface {
	// CreateNotifier adds a rule at version 1.
	CreateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// LoadNotifier includes deleted rules. Check DeletedAt before using one.
	LoadNotifier(ctx context.Context, tenant TenantID, pipelineID, id string) (Notifier, error)
	// ListNotifiers returns matching rules ordered by ID.
	ListNotifiers(ctx context.Context, f NotifierFilter) ([]Notifier, error)
	// UpdateNotifier updates a rule and increments Version. Its ID, tenant,
	// pipeline, and type cannot change. Clean up old secrets only after success.
	UpdateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// DeleteNotifier marks a rule deleted and increments Version.
	// It returns the saved rule, including secret references for cleanup.
	DeleteNotifier(ctx context.Context, tenant TenantID, pipelineID, id string, version int64) (Notifier, error)
}
