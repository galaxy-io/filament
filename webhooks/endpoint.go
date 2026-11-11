package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"
	"unicode"

	"github.com/galaxy-io/filament"
)

const (
	// DefaultMaxBody is the default maximum webhook body size in bytes.
	DefaultMaxBody = 1 << 20
	// HardMaxBody is the largest webhook body any endpoint may accept.
	HardMaxBody = 16 << 20
	// DefaultSuccessStatus is returned after a delivery is durably accepted.
	DefaultSuccessStatus = http.StatusAccepted
	// SecondaryKeyGrace is how long a replaced signing key keeps verifying.
	SecondaryKeyGrace = 72 * time.Hour
	maxResourceName   = 64
)

var (
	// ErrRouteNotReady indicates that the endpoint's route names a pipeline that does not exist.
	ErrRouteNotReady = errors.New("webhook route names a pipeline that does not exist")
	// ErrInvalidEndpoint indicates that an endpoint mutation failed validation.
	ErrInvalidEndpoint = errors.New("invalid webhook endpoint configuration")
)

// Endpoint is tenant-owned receiver configuration keyed by source connection.
// Resources are the names it accepts deliveries for. Where each one is written
// is the consuming pipeline's decision. SecretRefs hold the primary key first
// and, during rotation, the previous key.
type Endpoint struct {
	Tenant     filament.TenantID
	SourceID   string
	Verifier   string
	Enabled    bool
	Revision   int64
	PipelineID string
	RouteKey   string
	RouteReady bool
	// Accepting is true while a pipeline that reads this source is running or
	// paused. When every reader is stopped, new deliveries wait with the sender.
	Accepting             bool
	Resources             []string
	SecretRefs            []string
	SecondaryKeyExpiresAt time.Time
	MaxBodyBytes          int64
	SuccessStatus         int
}

// Accepts reports whether the endpoint takes new deliveries for resource.
func (e Endpoint) Accepts(resource string) bool {
	return e.Enabled && slices.Contains(e.Resources, resource)
}

// ActiveKeyRefs returns the key references that still verify at now.
func (e Endpoint) ActiveKeyRefs(now time.Time) []string {
	if len(e.SecretRefs) > 1 && !e.SecondaryKeyExpiresAt.IsZero() && !now.Before(e.SecondaryKeyExpiresAt) {
		return e.SecretRefs[:1]
	}
	return e.SecretRefs
}

// EndpointMutation is a tenant-scoped, revision-fenced configuration write.
// The caller stores signing secrets before persisting their references.
type EndpointMutation struct {
	Tenant                filament.TenantID
	SourceID              string
	Verifier              string
	PipelineID            string
	RouteKey              string
	Resources             []string
	Enabled               bool
	SecretRefs            []string
	SecondaryKeyExpiresAt time.Time
	SuccessStatus         int
	MaxBodyBytes          int64
	ExpectedRevision      int64
}

// ApplyDefaults fills the values a caller may omit.
func (m *EndpointMutation) ApplyDefaults() {
	if m.Verifier == "" {
		m.Verifier = GenericHMAC
	}
	if m.SuccessStatus == 0 {
		m.SuccessStatus = DefaultSuccessStatus
	}
	if m.MaxBodyBytes == 0 {
		m.MaxBodyBytes = DefaultMaxBody
	}
	if m.Resources == nil {
		m.Resources = []string{}
	}
	slices.Sort(m.Resources)
}

// Validate is the single definition of a well-formed endpoint. The schema's
// CHECK constraints repeat it as a backstop.
func (m EndpointMutation) Validate() error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidEndpoint, reason) }
	switch {
	case m.Tenant == "" || m.SourceID == "":
		return invalid("tenant and source are required")
	case m.ExpectedRevision < 0:
		return invalid("expected revision is negative")
	case (m.PipelineID == "") != (m.RouteKey == ""):
		return invalid("pipeline and route are set together")
	case m.Enabled && (m.PipelineID == "" || len(m.Resources) == 0):
		return invalid("an enabled endpoint needs a route and resources")
	case len(m.SecretRefs) == 0 || len(m.SecretRefs) > 2:
		return invalid("one or two signing keys are required")
	case m.MaxBodyBytes <= 0 || m.MaxBodyBytes > HardMaxBody:
		return invalid(fmt.Sprintf("max body must be between 1 and %d bytes", HardMaxBody))
	case !validSuccessStatus(m.SuccessStatus):
		return invalid("success status must be 200, 202 or 204")
	}
	if _, ok := LookupVerifier(m.Verifier); !ok {
		return invalid(fmt.Sprintf("unknown verifier %q", m.Verifier))
	}
	for n, name := range m.Resources {
		if !ValidResourceName(name) || slices.Contains(m.Resources[:n], name) {
			return invalid(fmt.Sprintf("invalid or repeated resource %q", name))
		}
	}
	for _, ref := range m.SecretRefs {
		if err := filament.ValidateConnectionSecretRef(ref, m.Tenant); err != nil {
			return invalid(err.Error())
		}
	}
	return nil
}

func validSuccessStatus(status int) bool {
	return status == http.StatusOK || status == http.StatusAccepted || status == http.StatusNoContent
}

// ValidResourceName reports whether a resource is safe in a webhook URL.
func ValidResourceName(name string) bool { return validToken(name, maxResourceName) }

func validToken(value string, maxLength int) bool {
	if value == "" || len(value) > maxLength {
		return false
	}
	for _, ch := range value {
		if ch > unicode.MaxASCII || (!unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-') {
			return false
		}
	}
	return true
}

// EndpointLoader resolves a persisted endpoint by source connection ID alone.
// Only the receiver uses it: the tenant comes from the stored row.
type EndpointLoader interface {
	LoadWebhookEndpoint(context.Context, string) (Endpoint, error)
}

// EndpointStore manages receiver configuration through the authenticated API.
type EndpointStore interface {
	LoadWebhookEndpointForTenant(context.Context, filament.TenantID, string) (Endpoint, error)
	PutWebhookEndpoint(context.Context, EndpointMutation) (Endpoint, error)
}
