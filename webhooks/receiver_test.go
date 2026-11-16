package webhooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

const receiverSource = "6f1c1f56-4bb6-4a8e-9c2f-0a4c9a3f1b11"

type stubResolver struct{ resolved Resolved }

func (s stubResolver) Resolve(context.Context, string) (Resolved, error) { return s.resolved, nil }

type stubInbox struct{ accepted []inbox.Delivery }

func (s *stubInbox) Accept(_ context.Context, d inbox.Delivery) (inbox.Receipt, error) {
	s.accepted = append(s.accepted, d)
	return inbox.Receipt{}, nil
}

func (s *stubInbox) Resolve(context.Context, string, string, string, []byte) (inbox.Receipt, error) {
	return inbox.Receipt{}, inbox.ErrNotFound
}

// githubLike signs the body and names the event in X-GitHub-Event.
const githubLike = `{"signature":{"header":"X-Hub-Signature-256","prefix":"sha256="},"delivery_id":{"header":"X-GitHub-Delivery"}}`

func newTestReceiver(t *testing.T, ingress Ingress, resources ...string) (*Receiver, *stubInbox) {
	t.Helper()
	verifier, err := BuildVerifier(HMAC, json.RawMessage(githubLike))
	if err != nil {
		t.Fatal(err)
	}
	if err := ingress.Validate(); err != nil {
		t.Fatal(err)
	}
	box := &stubInbox{}
	return &Receiver{
		Endpoints: stubResolver{Resolved{
			Endpoint: Endpoint{
				Tenant: filament.DefaultTenantID, SourceID: receiverSource, Verifier: HMAC,
				VerifierParams: json.RawMessage(githubLike), Ingress: ingress,
				Enabled: true, Accepting: true, Resources: resources, SuccessStatus: http.StatusAccepted,
			},
			Keys: [][]byte{[]byte("k")}, Verifier: verifier,
		}},
		Inbox: box, Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}, box
}

func signedRequest(method, path, body string, header map[string]string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(sign(sha256.New, []byte("k"), []byte(body))))
	for k, v := range header {
		r.Header.Set(k, v)
	}
	return r
}

func serve(h *Receiver, r *http.Request) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.Mount(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestReceiverRoutesByEventType(t *testing.T) {
	h, box := newTestReceiver(t, Ingress{Route: &Route{
		Event:  ValueSource{Header: "X-GitHub-Event"},
		Events: map[string]string{"push": "pushes", "issues": "issues"},
	}}, "pushes")

	w := serve(h, signedRequest(http.MethodPost, "/hooks/"+receiverSource, `{"ref":"main"}`,
		map[string]string{"X-GitHub-Event": "push", "X-GitHub-Delivery": "d1"}))
	if w.Code != http.StatusAccepted || len(box.accepted) != 1 || box.accepted[0].Resource != "pushes" {
		t.Fatalf("push: status %d, accepted %+v", w.Code, box.accepted)
	}

	// Mapped but not selected, and unmapped: acknowledged and dropped.
	for _, event := range []string{"issues", "star"} {
		w = serve(h, signedRequest(http.MethodPost, "/hooks/"+receiverSource, `{}`,
			map[string]string{"X-GitHub-Event": event, "X-GitHub-Delivery": "d-" + event}))
		if w.Code != http.StatusAccepted || len(box.accepted) != 1 {
			t.Fatalf("%s: status %d, accepted %d", event, w.Code, len(box.accepted))
		}
	}

	// An unsigned routed delivery is rejected before it is routed.
	r := httptest.NewRequest(http.MethodPost, "/hooks/"+receiverSource, strings.NewReader(`{}`))
	r.Header.Set("X-GitHub-Event", "push")
	r.Header.Set("X-GitHub-Delivery", "d2")
	if w = serve(h, r); w.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned: status %d", w.Code)
	}
}

func TestReceiverSingleURLNeedsRoute(t *testing.T) {
	h, box := newTestReceiver(t, Ingress{}, "pushes")
	w := serve(h, signedRequest(http.MethodPost, "/hooks/"+receiverSource, `{}`, map[string]string{"X-GitHub-Delivery": "d1"}))
	if w.Code != http.StatusNotFound || len(box.accepted) != 0 {
		t.Fatalf("status %d, accepted %d", w.Code, len(box.accepted))
	}
	w = serve(h, signedRequest(http.MethodPost, "/hooks/"+receiverSource+"/pushes", `{}`, map[string]string{"X-GitHub-Delivery": "d1"}))
	if w.Code != http.StatusAccepted || len(box.accepted) != 1 {
		t.Fatalf("resource path: status %d, accepted %d", w.Code, len(box.accepted))
	}
}

func TestIngressValidate(t *testing.T) {
	for name, ingress := range map[string]Ingress{
		"route without events": {Route: &Route{Event: ValueSource{Header: "X-Event"}}},
		"route two sources":    {Route: &Route{Event: ValueSource{Header: "X-Event", Body: "type"}, Events: map[string]string{"a": "a"}}},
		"route bad resource":   {Route: &Route{Event: ValueSource{Header: "X-Event"}, Events: map[string]string{"a": "a.b"}}},
	} {
		if err := ingress.Validate(); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
	raw := StoredIngress(Ingress{Route: &Route{Event: ValueSource{Header: "x-github-event"}, Events: map[string]string{"push": "pushes"}}})
	parsed, err := ParseIngress(raw)
	if err != nil || parsed.Route.Event.Header != "X-Github-Event" {
		t.Fatalf("round trip = %+v, %v", parsed.Route, err)
	}
	if parsed, err := ParseIngress([]byte("{}")); err != nil || !parsed.IsZero() {
		t.Fatalf("empty ingress = %+v, %v", parsed, err)
	}
}
