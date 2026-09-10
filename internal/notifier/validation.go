package notifier

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// NormalizeAndValidate returns a rule with sorted, unique filters. Supply events.Names() as eventNames.
// Destination validation and secret ownership checks belong to the selected channel and API.
func NormalizeAndValidate(n Notifier, eventNames []string) (Notifier, error) {
	if _, err := n.NotificationType.Label(); err != nil {
		return Notifier{}, fmt.Errorf("notifier: unsupported notification type")
	}
	n.Name = strings.TrimSpace(n.Name)
	if !utf8.ValidString(n.Name) {
		return Notifier{}, fmt.Errorf("notifier: name must be valid text")
	}
	n.Events = unique(n.Events)
	if len(n.Events) == 0 {
		return Notifier{}, fmt.Errorf("notifier: at least one event is required")
	}
	if slices.Contains(n.Events, "*") && len(n.Events) != 1 {
		return Notifier{}, fmt.Errorf("notifier: wildcard cannot be combined with event names")
	}
	for _, name := range n.Events {
		if name != "*" && (IsLifecycleEvent(name) || !slices.Contains(eventNames, name)) {
			return Notifier{}, fmt.Errorf("notifier: events must be registered ingestion event names, excluding notifier lifecycle events")
		}
	}
	n.Resources = unique(n.Resources)
	for _, resource := range n.Resources {
		if resource == "" || !utf8.ValidString(resource) {
			return Notifier{}, fmt.Errorf("notifier: resource filters must be non-empty valid text")
		}
	}
	return n, nil
}

// IsLifecycleEvent identifies notification reports, which must never trigger notifications.
func IsLifecycleEvent(name string) bool { return strings.HasPrefix(name, "notifier.") }

// Matches checks a live, enabled rule against an event name and exact resource name.
func Matches(n Notifier, eventName, resource string) bool {
	if !n.Enabled || n.DeletedAt != 0 || eventName == "" || IsLifecycleEvent(eventName) {
		return false
	}
	if !slices.Contains(n.Events, "*") && !slices.Contains(n.Events, eventName) {
		return false
	}
	return len(n.Resources) == 0 || resource != "" && slices.Contains(n.Resources, resource)
}

func unique(values []string) []string {
	out := append([]string{}, values...)
	slices.Sort(out)
	return slices.Compact(out)
}
