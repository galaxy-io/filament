package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
)

// notifierSecretRef builds the ref for a notifier-managed secret. Rules have no
// version, so each write gets a fresh revision.
func notifierSecretRef(tenant, notifierID, field string) string {
	return fmt.Sprintf("%s%s/notifier/%s/%s/%s", filament.ConnectionSecretPrefix, tenant, notifierID, field, uuid.NewString())
}

// validateNotifierConfig checks the config the rule will deliver with. A
// header secret the request left out is read back only when the reference is
// external; managed references were validated when they were written.
func (a *Server) validateNotifierConfig(ctx context.Context, tenant string, cfg map[string]any, refs map[string]string) error {
	effective := make(map[string]any, len(cfg))
	for k, v := range cfg {
		effective[k] = cloneConfigValue(v)
	}
	ref, hasRef := refs[webhook.HeadersField]
	if _, present := effective[webhook.HeadersField]; !present && hasRef && !strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
		if err := a.resolveNotifierSecret(ctx, tenant, webhook.HeadersField, ref, effective); err != nil {
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	if _, err := webhook.DestinationFromConfig(effective); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return nil
}

// resolveNotifierSecret reads one secret ref and injects its JSON object into cfg.
func (a *Server) resolveNotifierSecret(ctx context.Context, tenant, field, ref string, cfg map[string]any) error {
	if a.secrets == nil {
		return fmt.Errorf("secret ref for field %q supplied but no secret provider is configured", field)
	}
	secret, err := a.secrets.Read(ctx, ref)
	if err != nil || secret.Tenant != "" && secret.Tenant != filament.TenantID(tenant) {
		return fmt.Errorf("could not resolve secret for field %q", field)
	}
	var object map[string]any
	if err := json.Unmarshal(secret.Value, &object); err != nil {
		return fmt.Errorf("secret for field %q must be a JSON object", field)
	}
	cfg[field] = object
	return nil
}

// storeNotifierSecretFields mirrors storeSecretFields for rules: secret
// fields leave cfg and land in refs. An empty object clears the field.
func (a *Server) storeNotifierSecretFields(ctx context.Context, tenant, id string, cfg map[string]any, refs map[string]string, isUpdate bool) ([]string, error) {
	fields, err := extractSecretFields(webhook.ConfigSchema.Fields, cfg, "", isUpdate)
	if err != nil {
		return nil, err
	}
	written := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.value == "" {
			delete(refs, field.path)
			continue
		}
		if a.secrets == nil {
			return nil, fmt.Errorf("secret field %q supplied but no secret provider is configured", field.path)
		}
		ref := notifierSecretRef(tenant, id, field.path)
		secret := filament.Secret{
			Tenant: filament.TenantID(tenant),
			Value:  []byte(field.value),
			Meta:   map[string]string{"tenant": tenant, "notifier": id, "field": field.path},
		}
		if err := a.secrets.Write(ctx, ref, secret); err != nil {
			a.deleteSecretRefs(ctx, written)
			return nil, fmt.Errorf("store secret field %q: %w", field.path, err)
		}
		refs[field.path] = ref
		written = append(written, ref)
	}
	return written, nil
}

func (a *Server) deletePipelineNotifierSecrets(ctx context.Context, tenant filament.TenantID, pipelineID string) {
	if a.secrets == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// Read after pipeline deletion commits so concurrent notifier updates cannot
	// introduce references that cleanup misses.
	rules, err := a.store.ListNotifiers(ctx, tenant, pipelineID, true)
	if err != nil {
		if a.log != nil {
			a.log.Warn("could not list notifier secrets for cleanup",
				filament.Field{Key: "tenant.id", Value: tenant},
				filament.Field{Key: "pipeline.id", Value: pipelineID})
		}
		return
	}
	for _, n := range rules {
		a.deleteSecretRefs(ctx, mapValues(n.GetSecretRefs()))
	}
}
