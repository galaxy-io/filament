package secrets

import (
	"errors"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/oauth"
)

// Configurable identifies a saved refresh-token field and a stable fingerprint
// of its authorization settings. An empty field means refresh is not configured.
type Configurable interface {
	OAuthRefreshConfig(filament.Config) (field, binding string)
	SetOAuthStore(oauth.Store)
}

// Bind connects a source to its own saved refresh secret. It performs no reads
// or token exchanges. The first successful refresh converts the initial secret
// into encrypted token state under the same reference, so replacement and
// deletion keep using the connection's existing secret lifecycle.
func Bind(source any, cfg filament.Config, provider filament.Secrets, tenant filament.TenantID, refs map[string]string) error {
	connector, ok := source.(Configurable)
	if !ok {
		return nil
	}
	field, binding := connector.OAuthRefreshConfig(cfg)
	if field == "" {
		return nil
	}
	store, err := FromRefreshToken(provider, tenant, refs[field], binding)
	if err != nil {
		return err
	}
	connector.SetOAuthStore(store)
	return nil
}

// FromRefreshToken binds a connection-managed refresh-token secret. Initial
// values are opaque refresh tokens. After exchange, the value holds encrypted
// token state tagged in metadata. Binding identifies the provider, account and
// client settings, preventing reuse after those settings change.
func FromRefreshToken(provider filament.Secrets, tenant filament.TenantID, ref, binding string) (*Store, error) {
	if tenant == "" || binding == "" || !strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
		return nil, errors.New("oauth: automatic refresh requires a saved connection refresh-token secret")
	}
	if err := filament.ValidateConnectionSecretRef(ref, tenant); err != nil {
		return nil, err
	}
	u, ok := provider.(updater)
	if !ok {
		return nil, errors.New("oauth: secret provider does not support atomic token updates")
	}
	return &Store{provider: provider, updater: u, tenant: tenant, ref: ref, binding: binding}, nil
}
