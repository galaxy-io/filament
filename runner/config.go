package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// ResolveConfigRefs resolves opaque config and field references into the
// process-local RunSpec copy immediately before connector configuration.
func ResolveConfigRefs(ctx context.Context, secrets filament.Secrets, spec *filament.RunSpec) error {
	if err := resolveRefConfig(ctx, secrets, &spec.Source, spec.Tenant, "source"); err != nil {
		return err
	}
	if err := resolveRefConfig(ctx, secrets, &spec.Sink, spec.Tenant, "sink"); err != nil {
		return err
	}
	if err := resolveSecretRefs(ctx, secrets, &spec.Source, spec.Tenant, "source"); err != nil {
		return err
	}
	return resolveSecretRefs(ctx, secrets, &spec.Sink, spec.Tenant, "sink")
}

// resolveSecretRefs reads each of the ref's declared secrets and injects the
// plaintext value into the provider config under the mapped field. Every ref is
// tenant-scoped first, so a spec cannot read another tenant's connection secrets.
func resolveSecretRefs(ctx context.Context, secrets filament.Secrets, ref *filament.Ref, tenant filament.TenantID, role string) error {
	if len(ref.SecretRefs) == 0 {
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("%s %q has secret refs but no secrets store is configured", role, ref.Provider)
	}
	if ref.Config == nil {
		ref.Config = make(map[string]any, len(ref.SecretRefs))
	}
	for field, name := range ref.SecretRefs {
		if err := filament.ValidateConnectionSecretRef(name, tenant); err != nil {
			return fmt.Errorf("resolve %s secret for field %q: %w", role, field, err)
		}
		secret, err := secrets.Read(ctx, name)
		if err != nil {
			return fmt.Errorf("resolve %s secret %q for field %q: %w", role, name, field, err)
		}
		setConfigPath(ref.Config, field, string(secret.Value))
	}
	return nil
}

// setConfigPath assigns value at a dotted field path, rebuilding objects that
// were removed when their only submitted values were secrets.
func setConfigPath(cfg map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := cfg
	for _, part := range parts[:len(parts)-1] {
		nested, ok := current[part].(map[string]any)
		if !ok {
			nested = make(map[string]any)
			current[part] = nested
		}
		current = nested
	}
	current[parts[len(parts)-1]] = value
}

func resolveRefConfig(ctx context.Context, secrets filament.Secrets, ref *filament.Ref, tenant filament.TenantID, role string) error {
	if ref.ConfigRef == "" || len(ref.Config) > 0 {
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("%s %q has config ref %q but no secrets store is configured", role, ref.Provider, ref.ConfigRef)
	}
	if err := filament.ValidateConnectionSecretRef(ref.ConfigRef, tenant); err != nil {
		return fmt.Errorf("read %s config ref: %w", role, err)
	}
	secret, err := secrets.Read(ctx, ref.ConfigRef)
	if err != nil {
		return fmt.Errorf("read %s config ref %q: %w", role, ref.ConfigRef, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(secret.Value, &cfg); err != nil {
		return fmt.Errorf("decode %s config ref %q as JSON object: %w", role, ref.ConfigRef, err)
	}
	ref.Config = cfg
	return nil
}
