package filament

import (
	"context"
	"encoding/json"
	"fmt"
)

// NotificationType identifies the delivery channel of a pipeline notifier.
// Its integer values are domain constants; persistence and JSON use labels.
type NotificationType int

const (
	// NotificationUnspecified is an invalid input sentinel, never persisted.
	NotificationUnspecified NotificationType = 0
	// NotificationWebhook delivers an event to an HTTP endpoint.
	NotificationWebhook NotificationType = 1
)

// Label returns the database and JSON spelling of a supported notification
// type. Unspecified and unknown values are errors rather than implicit defaults.
func (t NotificationType) Label() (string, error) {
	switch t {
	case NotificationWebhook:
		return "webhook", nil
	default:
		return "", fmt.Errorf("invalid notification type %d", t)
	}
}

// ParseNotificationType converts a database or JSON label to its domain value.
func ParseNotificationType(label string) (NotificationType, error) {
	switch label {
	case "webhook":
		return NotificationWebhook, nil
	default:
		return NotificationUnspecified, fmt.Errorf("invalid notification type %q", label)
	}
}

// MarshalJSON serializes a notification type as its lowercase label.
func (t NotificationType) MarshalJSON() ([]byte, error) {
	label, err := t.Label()
	if err != nil {
		return nil, err
	}
	return json.Marshal(label)
}

// UnmarshalJSON accepts only a supported string label, leaving the receiver
// unchanged on invalid input. Numbers, null, and unspecified are rejected.
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

// Notifier is one tenant-owned pipeline notification rule. Config contains
// only non-secret settings; SecretRefs holds opaque provider references.
// Notifiers are mutable metadata, independent of pipeline graph versions.
type Notifier struct {
	ID               string
	Tenant           TenantID
	PipelineID       string
	Name             string
	NotificationType NotificationType
	Enabled          bool
	Events           []string
	Resources        []string
	Config           map[string]any
	SecretRefs       map[string]string
	Version          int64
	// Timestamps are Unix milliseconds; DeletedAt is zero for a live row.
	CreatedAt       int64
	UpdatedAt       int64
	DeletedAt       int64
	CreatedByUserID string
	UpdatedByUserID string
	DeletedByUserID string
}

// NotifierFilter selects one pipeline's rules. Disabled rules are included;
// soft-deleted rules are included only when explicitly requested for cleanup.
type NotifierFilter struct {
	Tenant         TenantID
	PipelineID     string
	IncludeDeleted bool
}

// NotifierStore is the optional persistence capability for pipeline
// notifications. It is deliberately separate from DataStore: providers that
// do not support notifications need not implement it.
//
// All operations require tenant and pipeline scope. Mutations must recheck
// parent liveness transactionally. The store persists opaque references only;
// secret-provider writes and cleanup belong to the API layer.
type NotifierStore interface {
	// CreateNotifier creates a version-1 rule on a live pipeline, enforcing the
	// limit of eight non-deleted rules (including disabled ones) atomically.
	CreateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// LoadNotifier includes soft-deleted rules so callers can inspect metadata
	// for cleanup. Callers must check DeletedAt before mutation or delivery.
	LoadNotifier(ctx context.Context, tenant TenantID, pipelineID, id string) (Notifier, error)
	// ListNotifiers returns rules in deterministic ID order within the pipeline.
	ListNotifiers(ctx context.Context, f NotifierFilter) ([]Notifier, error)
	// UpdateNotifier replaces mutable fields if n.Version matches the stored
	// version, returning the incremented record or ErrVersionConflict. Identity,
	// ownership, and notification type are immutable. Callers retain the loaded
	// old record to clean up replaced refs only after this compare-and-swap wins.
	UpdateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// DeleteNotifier soft-deletes a live rule at the expected version, returning
	// its committed metadata (including secret refs) for post-commit cleanup.
	// A stale version returns ErrVersionConflict. Deletion increments Version.
	DeleteNotifier(ctx context.Context, tenant TenantID, pipelineID, id string, version int64) (Notifier, error)
}
