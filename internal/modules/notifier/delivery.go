package notifier

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	notification "github.com/galaxy-io/filament/internal/notifier"
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
func (m *Module) deliverAll(ctx context.Context, trigger notification.Notification, rules []notification.Notifier) error {
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

// deliverWithRetry makes up to three attempts, then moves on even if delivery failed.
func (m *Module) deliverWithRetry(ctx context.Context, trigger notification.Notification, rule notification.Notifier) {
	for _, delay := range []time.Duration{0, time.Second, 2 * time.Second} {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
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
	}
}

func (m *Module) deliver(ctx context.Context, trigger notification.Notification, rule notification.Notifier) (out attempt) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	started := time.Now()
	out.notification = trigger
	out.notification.NotifierID = rule.ID
	out.notification.NotifierVersion = rule.Version
	out.notification.NotificationType = rule.NotificationType
	out.notification.AttemptID = uuid.NewString()
	out.result = notification.DeliveryResult{Outcome: notification.OutcomeFailed, Retryable: true}
	defer func() {
		out.completedAt = time.Now()
		out.result.Duration = time.Since(started)
		out.notification.Config = nil
	}()
	id, err := notification.DeliveryID(rule.ID, trigger.TriggerSubject, trigger.TriggerStreamSequence)
	if err != nil {
		out.result.ErrorCode = notification.ErrorInternal
		return out
	}
	out.notification.DeliveryID = id
	rule, config, err := m.resolveDestination(ctx, trigger, rule)
	out.notification.NotifierVersion = rule.Version
	out.notification.NotificationType = rule.NotificationType
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
	sender := m.senders[rule.NotificationType]
	if _, err := rule.NotificationType.Label(); err != nil || sender == nil {
		out.result.ErrorCode = notification.ErrorInvalidConfiguration
		return out
	}
	if ctx.Err() != nil {
		out.result.ErrorCode = notification.ErrorTimeout
		return out
	}
	out.notification.Config = config
	out.result, err = sender.Send(ctx, out.notification)
	if err != nil {
		out.result.Outcome = notification.OutcomeFailed
		if out.result.RequestAttempted {
			out.result.Outcome = notification.OutcomeUnknown
		}
		out.result.Retryable = true
		out.result.ErrorCode = notification.ErrorInternal
	}
	return out
}

func (m *Module) resolveDestination(ctx context.Context, trigger notification.Notification, rule notification.Notifier) (notification.Notifier, []byte, error) {
	ref := rule.SecretRefs["destination"]
	value, err := m.readDestination(ctx, rule, ref)
	if !errors.Is(err, filament.ErrNotFound) || !strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
		return rule, value, err
	}
	// A concurrent update may have replaced and removed the selected secret.
	current, loadErr := m.store.LoadNotifier(ctx, trigger.Tenant, trigger.PipelineID, rule.ID)
	if errors.Is(loadErr, filament.ErrNotFound) {
		return rule, nil, errInactiveRule
	}
	if loadErr != nil {
		return rule, nil, fmt.Errorf("could not reload notifier")
	}
	if current.Tenant != trigger.Tenant || current.PipelineID != trigger.PipelineID || current.ID != rule.ID {
		return rule, nil, errInvalidConfiguration
	}
	if !notification.Matches(current, trigger.TriggerType, trigger.Resource) {
		return current, nil, errInactiveRule
	}
	if current.Version == rule.Version {
		return current, nil, err
	}
	value, err = m.readDestination(ctx, current, current.SecretRefs["destination"])
	return current, value, err
}

func (m *Module) readDestination(ctx context.Context, rule notification.Notifier, ref string) ([]byte, error) {
	if ref == "" || filament.ValidateConnectionSecretRef(ref, rule.Tenant) != nil {
		return nil, errInvalidConfiguration
	}
	if m.secrets == nil {
		return nil, fmt.Errorf("secrets provider is unavailable")
	}
	secret, err := m.secrets.Read(ctx, ref)
	if err != nil {
		return nil, err
	}
	if secret.Tenant != "" && secret.Tenant != rule.Tenant {
		return nil, errInvalidConfiguration
	}
	return secret.Value, nil
}
