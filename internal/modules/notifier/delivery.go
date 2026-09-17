package notifier

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	notification "github.com/galaxy-io/filament/internal/notifier"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
)

var (
	errInactiveRule         = errors.New("notifier is no longer active")
	errInvalidConfiguration = errors.New("invalid notifier configuration")
)

type attempt struct {
	notification notification.Notification
	result       notification.DeliveryResult
	completedAt  time.Time
	skipped      bool
}

// deliverAll processes every matching rule, with at most eight in flight.
func (m *Module) deliverAll(ctx context.Context, trigger notification.Notification, rules []*ingestionv1.Notifier) error {
	var group errgroup.Group
	group.SetLimit(maxConcurrent)
	for _, rule := range rules {
		if ctx.Err() != nil {
			break
		}
		group.Go(func() error {
			m.deliverWithRetry(ctx, trigger, rule)
			return ctx.Err()
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}

// deliverWithRetry retries with exponential backoff up to maxAttempts, then
// moves on even if delivery failed.
func (m *Module) deliverWithRetry(ctx context.Context, trigger notification.Notification, rule *ingestionv1.Notifier) {
	var wait time.Duration
	for n := 0; n < maxAttempts; n++ {
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return
		}
		a := m.deliver(ctx, trigger, rule)
		if a.skipped {
			return
		}
		m.report(ctx, a)
		if !a.result.Retryable {
			return
		}
		wait = backoff(n+1, a.result.RetryAfter)
	}
}

// backoff returns the delay before retry n: backoffBase doubled per retry with
// equal jitter, raised to any Retry-After the endpoint asked for, and capped.
func backoff(n int, retryAfter time.Duration) time.Duration {
	d := backoffBase << (n - 1)
	if d > backoffCap || d <= 0 {
		d = backoffCap
	}
	upper := big.NewInt(int64(d/2 + 1))
	if jitter, err := rand.Int(rand.Reader, upper); err == nil {
		d = d/2 + time.Duration(jitter.Int64())
	}
	if retryAfter > d {
		d = retryAfter
	}
	if d > backoffCap {
		d = backoffCap
	}
	return d
}

func (m *Module) deliver(ctx context.Context, trigger notification.Notification, rule *ingestionv1.Notifier) (out attempt) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	started := time.Now()
	out.notification = trigger
	out.notification.NotifierID = rule.GetId()
	out.notification.NotificationType = rule.GetNotificationType()
	out.notification.AttemptID = uuid.NewString()
	out.result = notification.DeliveryResult{Outcome: notification.OutcomeFailed, Retryable: true}
	defer func() {
		out.completedAt = time.Now()
		out.result.Duration = time.Since(started)
		out.notification.Config = nil
	}()
	id, err := notification.DeliveryID(rule.GetId(), trigger.TriggerSubject, trigger.TriggerStreamSequence)
	if err != nil {
		out.result.ErrorCode = notification.ErrorInternal
		return out
	}
	out.notification.DeliveryID = id
	rule, config, err := m.resolveConfig(ctx, trigger, rule)
	out.notification.NotificationType = rule.GetNotificationType()
	if errors.Is(err, errInactiveRule) {
		out.skipped = true
		return out
	}
	if err != nil {
		out.result.ErrorCode = notification.ErrorSecretUnavailable
		if errors.Is(err, errInvalidConfiguration) {
			out.result.ErrorCode = notification.ErrorInvalidConfiguration
		}
		return out
	}
	sender := m.senders[rule.GetNotificationType()]
	if _, err := notification.NotificationTypeLabel(rule.GetNotificationType()); err != nil || sender == nil {
		out.result.ErrorCode = notification.ErrorInvalidConfiguration
		return out
	}
	if ctx.Err() != nil {
		out.result.ErrorCode = notification.ErrorTimeout
		return out
	}
	out.notification.Config = config
	out.result = sender.Send(ctx, out.notification)
	return out
}

// resolveConfig reads the rule's secret references into its config and
// renders the sender input. A managed reference that has vanished is retried
// against a reloaded rule, since a concurrent update may have replaced it.
func (m *Module) resolveConfig(ctx context.Context, trigger notification.Notification, rule *ingestionv1.Notifier) (*ingestionv1.Notifier, []byte, error) {
	config, err := m.renderConfig(ctx, rule)
	if !errors.Is(err, filament.ErrNotFound) {
		return rule, config, err
	}
	current, loadErr := m.ds.LoadNotifier(ctx, trigger.Tenant, trigger.PipelineID, rule.GetId())
	if errors.Is(loadErr, filament.ErrNotFound) {
		return rule, nil, errInactiveRule
	}
	if loadErr != nil {
		return rule, nil, fmt.Errorf("reload notifier: %w", loadErr)
	}
	if filament.TenantID(current.GetTenantId()) != trigger.Tenant || current.GetPipelineId() != trigger.PipelineID || current.GetId() != rule.GetId() {
		return rule, nil, errInvalidConfiguration
	}
	if !notification.Matches(current, trigger.TriggerEvent, trigger.Resource) {
		return current, nil, errInactiveRule
	}
	if maps.Equal(current.GetSecretRefs(), rule.GetSecretRefs()) {
		return current, nil, err
	}
	config, err = m.renderConfig(ctx, current)
	return current, config, err
}

// renderConfig resolves the header secret into the config and encodes the
// destination the sender expects.
func (m *Module) renderConfig(ctx context.Context, rule *ingestionv1.Notifier) ([]byte, error) {
	cfg := rule.GetConfig().AsMap()
	if cfg == nil {
		cfg = map[string]any{}
	}
	if ref := rule.GetSecretRefs()[webhook.HeadersField]; ref != "" {
		value, err := m.readSecret(ctx, rule, ref)
		if err != nil {
			return nil, err
		}
		var headers map[string]any
		if err := json.Unmarshal(value, &headers); err != nil {
			return nil, errInvalidConfiguration
		}
		cfg[webhook.HeadersField] = headers
	}
	destination, err := webhook.DestinationFromConfig(cfg)
	if err != nil {
		return nil, errInvalidConfiguration
	}
	raw, err := json.Marshal(destination)
	if err != nil {
		return nil, errInvalidConfiguration
	}
	return raw, nil
}

func (m *Module) readSecret(ctx context.Context, rule *ingestionv1.Notifier, ref string) ([]byte, error) {
	tenant := filament.TenantID(rule.GetTenantId())
	if ref == "" || filament.ValidateConnectionSecretRef(ref, tenant) != nil {
		return nil, errInvalidConfiguration
	}
	if m.secrets == nil {
		return nil, errors.New("secrets provider is unavailable")
	}
	secret, err := m.secrets.Read(ctx, ref)
	if err != nil {
		return nil, err
	}
	if secret.Tenant != "" && secret.Tenant != tenant {
		return nil, errInvalidConfiguration
	}
	return secret.Value, nil
}
