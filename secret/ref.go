package secret

import (
	"context"
	"fmt"
)

// Ref is the config-level reference to a secret value, Helm/configmap style, so
// credentials never live in a config file. Every component's config embeds this
// shape and resolves it locally through a Provider.
type Ref struct {
	Key Key `yaml:"secretKey"`
}

// Key names the secret and the field within it to read.
type Key struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Resolve reads the value a Ref points at through the provider. The reference is
// "name/value" (for example "postgres-creds/dsn"); each provider maps that to
// its own backend.
func Resolve(ctx context.Context, p Provider, ref Ref) (string, error) {
	key := ref.Key.Name
	if ref.Key.Value != "" {
		key += "/" + ref.Key.Value
	}
	v, err := p.Read(ctx, key)
	if err != nil {
		return "", fmt.Errorf("resolve secret %q: %w", key, err)
	}
	return string(v.Bytes), nil
}
