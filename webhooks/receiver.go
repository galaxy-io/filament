// Package webhooks accepts authenticated webhook deliveries independently of
// workers. A delivery is acknowledged to its sender only once the inbox holds it.
package webhooks

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

// Path is the public delivery route. Expose only this prefix to senders.
const Path = "POST /hooks/{sourceID}/{resource}"

// EndpointResolver returns an endpoint with its active signing keys.
type EndpointResolver interface {
	Resolve(context.Context, string) (Resolved, error)
}

// Inbox stores accepted deliveries and finds the originals of retries.
type Inbox interface {
	Accept(context.Context, inbox.Delivery) (inbox.Receipt, error)
	Resolve(ctx context.Context, tenant, endpoint, providerID string, body []byte) (inbox.Receipt, error)
}

// Receiver serves the delivery route on the server's listener.
type Receiver struct {
	Endpoints EndpointResolver
	Inbox     Inbox
	Now       func() time.Time
	// Slots bounds concurrent deliveries. A full receiver answers 503.
	Slots   chan struct{}
	Metrics filament.Metrics
}

// Mount adds the delivery route to mux.
func (h *Receiver) Mount(mux *http.ServeMux) { mux.Handle(Path, h) }

// ServeHTTP authenticates the bounded body, then stores it before answering.
// An enabled endpoint stores new deliveries for its configured resources while
// a pipeline can read them. When every reading pipeline is stopped, a new
// delivery gets 503 so the sender retries later instead of filling the inbox.
// Any other authenticated request only resolves a retry of an earlier delivery.
func (h *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	outcome := h.serve(w, r)
	if h.Metrics != nil {
		h.Metrics.Counter("filament_webhook_requests_total", filament.Label{Key: "outcome", Value: outcome}).Inc()
	}
}

func (h *Receiver) serve(w http.ResponseWriter, r *http.Request) string {
	if h.Slots != nil {
		select {
		case h.Slots <- struct{}{}:
			defer func() { <-h.Slots }()
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "webhook receiver busy", http.StatusServiceUnavailable)
			return "busy"
		}
	}
	sourceID, resource := r.PathValue("sourceID"), r.PathValue("resource")
	if _, err := uuid.Parse(sourceID); err != nil || !ValidResourceName(resource) {
		http.NotFound(w, r)
		return "not_found"
	}
	resolved, err := h.Endpoints.Resolve(r.Context(), sourceID)
	if errors.Is(err, filament.ErrNotFound) {
		http.NotFound(w, r)
		return "not_found"
	}
	if err != nil {
		http.Error(w, "endpoint unavailable", http.StatusServiceUnavailable)
		return "unavailable"
	}
	endpoint := resolved.Endpoint
	verifier, ok := LookupVerifier(endpoint.Verifier)
	if !ok {
		http.Error(w, "endpoint unavailable", http.StatusServiceUnavailable)
		return "unavailable"
	}
	limit := endpoint.MaxBodyBytes
	if limit <= 0 || limit > HardMaxBody {
		limit = DefaultMaxBody
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return "invalid"
	}
	if int64(len(body)) > limit {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return "too_large"
	}
	deliveryID, err := verifier.Verify(r.Header, body, resolved.Keys, h.now())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "unauthorized"
	}
	tenant := string(endpoint.Tenant)
	if !endpoint.Accepts(resource) || !endpoint.Accepting {
		_, err := h.Inbox.Resolve(r.Context(), tenant, endpoint.SourceID, deliveryID, body)
		if errors.Is(err, inbox.ErrNotFound) && endpoint.Accepts(resource) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "no pipeline is reading this endpoint", http.StatusServiceUnavailable)
			return "no_consumer"
		}
		return h.answer(w, r, endpoint, err, true)
	}
	_, err = h.Inbox.Accept(r.Context(), inbox.Delivery{
		Tenant: tenant, Endpoint: endpoint.SourceID,
		ProviderDeliveryID: deliveryID, Resource: resource, Body: body,
	})
	return h.answer(w, r, endpoint, err, false)
}

func (h *Receiver) answer(w http.ResponseWriter, r *http.Request, endpoint Endpoint, err error, retryOnly bool) string {
	switch {
	case err == nil:
		status := endpoint.SuccessStatus
		if !validSuccessStatus(status) {
			status = DefaultSuccessStatus
		}
		w.WriteHeader(status)
		if retryOnly {
			return "duplicate"
		}
		return "accepted"
	case retryOnly && errors.Is(err, inbox.ErrNotFound):
		http.NotFound(w, r)
		return "not_found"
	case errors.Is(err, inbox.ErrConflict):
		http.Error(w, "conflicting delivery ID", http.StatusConflict)
		return "conflict"
	case errors.Is(err, inbox.ErrTooLarge):
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return "too_large"
	case errors.Is(err, inbox.ErrInvalid):
		http.Error(w, "invalid delivery", http.StatusBadRequest)
		return "invalid"
	default:
		http.Error(w, "inbox unavailable", http.StatusServiceUnavailable)
		return "unavailable"
	}
}

func (h *Receiver) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}
