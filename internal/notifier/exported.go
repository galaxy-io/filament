package notifier

import (
	"fmt"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/events"
)

// exported ties each deliverable event kind to its catalog event, in enum
// order. The catalog stays internal; each kind here has a webhook shape in
// Project.
var exported = []struct {
	event ingestionv1.NotifierEvent
	name  string
}{
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_STARTED, events.RunStarted.Name()},
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_COMPLETED, events.RunCompleted.Name()},
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_FAILED, events.RunFailed.Name()},
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_PARTIAL, events.RunPartial.Name()},
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_CANCELED, events.RunCanceled.Name()},
	{ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_PAUSED, events.RunPaused.Name()},
}

// Exported returns the catalog definition of every deliverable event kind.
func Exported() []events.Definition {
	out := make([]events.Definition, 0, len(exported))
	for _, e := range exported {
		d, ok := events.Lookup(e.name)
		if !ok {
			panic(fmt.Sprintf("notifier: exported event %q is not in the catalog", e.name))
		}
		out = append(out, d)
	}
	return out
}

// EventName returns the catalog and database name for an event kind.
func EventName(e ingestionv1.NotifierEvent) (string, error) {
	for _, x := range exported {
		if x.event == e {
			return x.name, nil
		}
	}
	return "", fmt.Errorf("invalid notifier event %d", e)
}

// ParseEventName reads a catalog name such as "run.completed".
func ParseEventName(name string) (ingestionv1.NotifierEvent, error) {
	for _, x := range exported {
		if x.name == name {
			return x.event, nil
		}
	}
	return ingestionv1.NotifierEvent_NOTIFIER_EVENT_UNSPECIFIED, fmt.Errorf("invalid notifier event %q", name)
}
