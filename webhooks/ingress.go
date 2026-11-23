package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Ingress tells the receiver how to handle a delivery beyond verifying it.
// Today that is how to find the resource of a delivery sent to the endpoint's
// single URL. A connector declares it as data, saving an endpoint copies it
// like the verifier parameters, and the store keeps it as one JSON object, so
// a new kind of ingress handling is a new field here and needs no migration.
type Ingress struct {
	Route *Route `json:"route,omitempty"`
}

// RouteAny maps every event type that has no resource of its own.
const RouteAny = "*"

// Route finds the resource of a delivery sent to the endpoint's single URL,
// for providers that send every event type to one URL and name the type in
// a header or the body. Events maps each event type to the resource that
// reads it. RouteAny catches the rest. A delivery whose type maps to nothing
// is acknowledged and dropped, so the sender does not retry it.
type Route struct {
	Event  ValueSource       `json:"event"`
	Events map[string]string `json:"events"`
}

// Resolve returns the resource that reads a delivery.
func (r *Route) Resolve(header http.Header, body []byte) (string, bool) {
	event, ok := r.Event.lookup(header, body)
	if !ok {
		return "", false
	}
	if resource, ok := r.Events[event]; ok {
		return resource, true
	}
	resource, ok := r.Events[RouteAny]
	return resource, ok
}

func (r *Route) validate() error {
	if err := r.Event.validate("route event"); err != nil {
		return err
	}
	if len(r.Events) == 0 {
		return errors.New("route maps no events")
	}
	for event, resource := range r.Events {
		if event == "" {
			return errors.New("route maps an empty event type")
		}
		if !ValidResourceName(resource) {
			return fmt.Errorf("route maps event %q to invalid resource %q", event, resource)
		}
	}
	return nil
}

// Validate checks the ingress configuration.
func (i Ingress) Validate() error {
	if i.Route != nil {
		return i.Route.validate()
	}
	return nil
}

// IsZero reports whether the ingress configures nothing.
func (i Ingress) IsZero() bool { return i.Route == nil }

// ParseIngress decodes stored ingress configuration. Empty input is none.
func ParseIngress(raw []byte) (Ingress, error) {
	var out Ingress
	if emptyParams(raw) {
		return out, nil
	}
	// Stored rows decode leniently: a server rolled back past a newer
	// ingress field still loads the endpoint and ignores the field.
	if err := json.Unmarshal(raw, &out); err != nil {
		return Ingress{}, fmt.Errorf("webhook ingress: %w", err)
	}
	if out.Route != nil {
		out.Route.Event = out.Route.Event.canonical()
	}
	return out, out.Validate()
}

// StoredIngress encodes ingress configuration as the store keeps it: a JSON
// object, "{}" when there is none.
func StoredIngress(i Ingress) []byte {
	if i.IsZero() {
		return []byte("{}")
	}
	raw, err := json.Marshal(i)
	if err != nil {
		return []byte("{}")
	}
	return raw
}
