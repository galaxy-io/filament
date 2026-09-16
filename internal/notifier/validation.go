package notifier

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// NormalizeAndValidate returns a rule with sorted, unique events.
// Config validation and secret ownership checks belong to the selected channel and API.
func NormalizeAndValidate(n *ingestionv1.Notifier) (*ingestionv1.Notifier, error) {
	if n == nil {
		return nil, fmt.Errorf("notifier: rule is required")
	}
	if _, err := NotificationTypeLabel(n.GetNotificationType()); err != nil {
		return nil, fmt.Errorf("notifier: %w", err)
	}
	n.Name = strings.TrimSpace(n.Name)
	if !utf8.ValidString(n.Name) {
		return nil, fmt.Errorf("notifier: name must be valid text")
	}
	events := slices.Clone(n.Events)
	slices.Sort(events)
	n.Events = slices.Compact(events)
	if len(n.Events) == 0 {
		return nil, fmt.Errorf("notifier: at least one event is required")
	}
	for _, event := range n.Events {
		if _, err := EventName(event); err != nil {
			return nil, fmt.Errorf("notifier: %w", err)
		}
	}
	if len(n.GetResources()) != 0 {
		return nil, fmt.Errorf("notifier: resource filters are not supported for run events")
	}
	return n, nil
}

// Matches checks a live, enabled rule against an event kind and exact resource name.
func Matches(n *ingestionv1.Notifier, event ingestionv1.NotifierEvent, resource string) bool {
	if n == nil || !n.GetIsEnabled() || n.GetDeletedAt() != 0 || event == ingestionv1.NotifierEvent_NOTIFIER_EVENT_UNSPECIFIED {
		return false
	}
	if !slices.Contains(n.GetEvents(), event) {
		return false
	}
	return len(n.GetResources()) == 0 || resource != "" && slices.Contains(n.GetResources(), resource)
}
