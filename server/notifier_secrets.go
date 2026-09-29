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
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier/slack"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
)

// notifierSecretRef builds the ref for a notifier-managed secret. Rules have no
// version, so each write gets a fresh revision.
func notifierSecretRef(tenant, notifierID, field string) string {
	return fmt.Sprintf("%s%s/notifier/%s/%s/%s", filament.ConnectionSecretPrefix, tenant, notifierID, field, uuid.NewString())
}

// notifierSchema returns the config fields of a notification type.
func notifierSchema(kind ingestionv1.NotificationType) filament.ConfigSchema {
	if kind == ingestionv1.NotificationType_NOTIFICATION_TYPE_SLACK {
		return slack.ConfigSchema
	}
	return webhook.ConfigSchema
}

// validateNotifierConfig checks the config the rule will deliver with. A
// secret the request left out is read back only when the reference is
// external; managed references were validated when they were written.
func (a *Server) validateNotifierConfig(ctx context.Context, tenant string, kind ingestionv1.NotificationType, cfg map[string]any, refs map[string]string) error {
	effective := make(map[string]any, len(cfg))
	for k, v := range cfg {
		effective[k] = cloneConfigValue(v)
	}
	for _, field := range notifierSchema(kind).Fields {
		ref, hasRef := refs[field.Name]
		if !field.Secret || !hasRef || submitted(effective[field.Name]) {
			continue
		}
		if strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
			continue
		}
		if err := a.resolveNotifierSecret(ctx, tenant, field, ref, effective); err != nil {
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	var err error
	switch kind {
	case ingestionv1.NotificationType_NOTIFICATION_TYPE_SLACK:
		if !submitted(effective[slack.URLField]) && strings.HasPrefix(refs[slack.URLField], filament.ConnectionSecretPrefix) {
			return nil
		}
		_, err = slack.DestinationFromConfig(effective)
	default:
		_, err = webhook.DestinationFromConfig(effective)
	}
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return nil
}

// submitted reports whether the request carried a value for a secret field.
// An empty string keeps the stored value, as it does when the field is stored.
func submitted(value any) bool {
	s, isString := value.(string)
	return value != nil && (!isString || s != "")
}

// resolveNotifierSecret reads one secret ref and injects its value into cfg.
func (a *Server) resolveNotifierSecret(ctx context.Context, tenant string, field filament.ConfigField, ref string, cfg map[string]any) error {
	if a.secrets == nil {
		return fmt.Errorf("secret ref for field %q supplied but no secret provider is configured", field.Name)
	}
	secret, err := a.secrets.Read(ctx, ref)
	if err != nil || secret.Tenant != "" && secret.Tenant != filament.TenantID(tenant) {
		return fmt.Errorf("could not resolve secret for field %q", field.Name)
	}
	if field.Type != filament.FieldObject {
		cfg[field.Name] = string(secret.Value)
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(secret.Value, &object); err != nil {
		return fmt.Errorf("secret for field %q must be a JSON object", field.Name)
	}
	cfg[field.Name] = object
	return nil
}

// storeNotifierSecretFields mirrors storeSecretFields for rules: secret
// fields leave cfg and land in refs. An empty object clears the field.
func (a *Server) storeNotifierSecretFields(ctx context.Context, tenant string, n *ingestionv1.Notifier, cfg map[string]any, refs map[string]string, isUpdate bool) ([]string, error) {
	fields, err := extractSecretFields(notifierSchema(n.GetNotificationType()).Fields, cfg, "", isUpdate)
	if err != nil {
		return nil, err
	}
	id := n.GetId()
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
