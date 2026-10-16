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

// Bind connects a source to its saved refresh secret with a new run-scoped store.
// It performs no reads or token exchanges.
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

// FromRefreshToken binds an existing connection secret. Only a changed refresh
// token is written back. Access tokens remain in this store's memory.
func FromRefreshToken(provider filament.Secrets, tenant filament.TenantID, ref, binding string) (*Store, error) {
	if tenant == "" || binding == "" || !strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
		return nil, errors.New("oauth: automatic refresh requires a saved connection refresh-token secret")
	}
	if err := filament.ValidateConnectionSecretRef(ref, tenant); err != nil {
		return nil, err
	}
	rotator, ok := provider.(filament.SecretRotator)
	if !ok {
		return nil, errors.New("oauth: secret provider does not support secret rotation")
	}
	return &Store{provider: provider, rotator: rotator, tenant: tenant, ref: ref, binding: binding, lock: make(chan struct{}, 1)}, nil
}
