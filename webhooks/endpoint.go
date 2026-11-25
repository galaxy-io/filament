package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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
	// PreviousKeyGrace is how long a replaced signing key keeps verifying.
	PreviousKeyGrace = 72 * time.Hour
	maxResourceName  = 64
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
	Tenant   filament.TenantID
	SourceID string
	Verifier string
	// VerifierParams configure the verifier, copied from the source
	// connector's webhook spec when the endpoint is saved. Empty for a
	// verifier that takes none.
	VerifierParams json.RawMessage
	// Ingress routes deliveries sent to the endpoint's single URL, copied
	// from the spec like VerifierParams.
	Ingress    Ingress
	Enabled    bool
	Revision   int64
	PipelineID string
	RouteKey   string
	RouteReady bool
	// Accepting is true once a stream reads this source, whether it is
	// running, paused or stopped. Receiving does not depend on processing.
	// With no stream to resume, new deliveries wait with the sender.
	Accepting            bool
	Resources            []string
	SecretRefs           []string
	PreviousKeyExpiresAt time.Time
	MaxBodyBytes         int64
	SuccessStatus        int
}

// Accepts reports whether the endpoint takes new deliveries for resource.
func (e Endpoint) Accepts(resource string) bool {
	return e.Enabled && slices.Contains(e.Resources, resource)
}

// ActiveKeyRefs returns the key references that still verify at now.
func (e Endpoint) ActiveKeyRefs(now time.Time) []string {
	if len(e.SecretRefs) > 1 && !e.PreviousKeyExpiresAt.IsZero() && !now.Before(e.PreviousKeyExpiresAt) {
		return e.SecretRefs[:1]
	}
	return e.SecretRefs
}

// EndpointMutation is a tenant-scoped, revision-fenced configuration write.
// The caller stores signing secrets before persisting their references.
type EndpointMutation struct {
	Tenant                    filament.TenantID
	SourceID                  string
	Verifier                  string
	VerifierParams            json.RawMessage
	Ingress                   Ingress
	PipelineID                string
	RouteKey                  string
	Resources                 []string
	Enabled                   bool
	SecretRefs                []string
	PreviousKeyExpiresAt      time.Time
	SuccessStatus             int
	MaxBodyBytes              int64
	ExpectedRevision          int64
	ExpectedPipelineVersionID string
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
	case m.ExpectedPipelineVersionID != "" && m.PipelineID == "":
		return invalid("expected pipeline version requires a pipeline")
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
	if _, err := BuildVerifier(m.Verifier, m.VerifierParams); err != nil {
		return invalid(err.Error())
	}
	if err := m.Ingress.Validate(); err != nil {
		return invalid(err.Error())
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

// StoredVerifierParams returns verifier parameters as the store keeps them:
// a JSON object, "{}" when there are none.
func StoredVerifierParams(params json.RawMessage) []byte {
	if emptyParams(params) {
		return []byte("{}")
	}
	return params
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

// EndpointVerification is the part of an endpoint copied from its source
// connector's webhook spec: how deliveries are verified and routed. The
// server compares it with the connector's current spec and rewrites it when
// a connector update changes either.
type EndpointVerification struct {
	Tenant         filament.TenantID
	SourceID       string
	Connector      string
	Revision       int64
	Verifier       string
	VerifierParams json.RawMessage
	Ingress        Ingress
}

// Matches reports whether the endpoint already verifies and routes as the
// spec says.
func (v EndpointVerification) Matches(verifier string, params json.RawMessage, ingress Ingress) bool {
	return v.Verifier == verifier &&
		jsonEqual(StoredVerifierParams(v.VerifierParams), StoredVerifierParams(params)) &&
		jsonEqual(StoredIngress(v.Ingress), StoredIngress(ingress))
}

// VerificationStore lists and rewrites endpoint verification. Updating checks
// Revision and returns filament.ErrVersionConflict when the endpoint changed.
type VerificationStore interface {
	ListWebhookEndpointVerifications(context.Context) ([]EndpointVerification, error)
	UpdateWebhookEndpointVerification(context.Context, EndpointVerification) error
}

// jsonEqual compares two JSON documents by value, so key order and spacing,
// which a JSONB column does not keep, do not count as a change.
func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(x, y)
}
