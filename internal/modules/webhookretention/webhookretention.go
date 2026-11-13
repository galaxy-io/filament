// Package webhookretention purges the webhook inbox. It runs in the control
// plane, which has exactly one replica, so one process sweeps the inbox.
//
// A delivery is purged once it is older than the retention period and every
// pipeline stream that could resume reading its source, whether running,
// paused or stopped, has certified it. Retention also bounds deduplication: a
// sender's retry of a purged delivery is accepted again.
//
// A delivery no stream has certified is kept for at most the hold period, so
// a paused or forgotten stopped pipeline cannot fill the shared inbox. Past
// it, the delivery is dropped, counted and logged. Endpoints with no stream
// that could resume, including deleted ones, are purged on age alone. Once a
// deleted source's deliveries are all gone, its endpoint row is removed too.
package webhookretention

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

const (
	defaultInterval  = time.Minute
	defaultRetention = 168 * time.Hour
	defaultHold      = 720 * time.Hour
)

// Attacher opens the inbox the server provisioned.
type Attacher func(context.Context) (*inbox.Inbox, func(), error)

// ReleaseStore reports how far each endpoint's live consumers have certified,
// and forgets endpoints of deleted sources.
type ReleaseStore interface {
	WebhookReleases(ctx context.Context, stream string, incarnation time.Time) ([]inbox.Release, error)
	DeleteWebhookEndpointsOfDeletedSources(ctx context.Context, before time.Time) (int64, error)
}

// receiverCacheLag covers a deleted endpoint still accepting from a receiver's
// cache, so its last delivery is older than the deletion plus this margin.
const receiverCacheLag = time.Minute

// Module is the retention sweep.
type Module struct {
	attach    Attacher
	retention time.Duration
	hold      time.Duration
	interval  time.Duration
	now       func() time.Time

	store ReleaseStore
	log   filament.Logger
	mx    filament.Metrics

	inbox *inbox.Inbox
	close func()
}

// New returns a sweep that keeps deliveries for at least retention and
// uncertified deliveries for at most hold. Hold must not be shorter than
// retention.
func New(attach Attacher, retention, hold time.Duration) (*Module, error) {
	if retention <= 0 || hold < retention {
		return nil, errors.New("webhook inbox hold must be at least the retention period")
	}
	return &Module{attach: attach, retention: retention, hold: hold, interval: defaultInterval, now: time.Now}, nil
}

// NewFromEnv reads WEBHOOK_INBOX_RETENTION_HOURS, 168 by default, and
// WEBHOOK_INBOX_MAX_HOLD_HOURS, 720 by default.
func NewFromEnv(attach Attacher) (*Module, error) {
	retention, err := envHours("WEBHOOK_INBOX_RETENTION_HOURS", defaultRetention)
	if err != nil {
		return nil, err
	}
	hold, err := envHours("WEBHOOK_INBOX_MAX_HOLD_HOURS", defaultHold)
	if err != nil {
		return nil, err
	}
	if hold < retention {
		return nil, errors.New("WEBHOOK_INBOX_MAX_HOLD_HOURS must be at least WEBHOOK_INBOX_RETENTION_HOURS")
	}
	return New(attach, retention, hold)
}

func envHours(name string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback, nil
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours < 1 || hours > 24*365 {
		return 0, fmt.Errorf("%s must be between 1 and 8760", name)
	}
	return time.Duration(hours) * time.Hour, nil
}

var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "webhookretention" }

// Subscriptions returns none; the sweep is timer-driven.
func (m *Module) Subscriptions() []host.Subscription { return nil }

// Mount captures the store, which must report webhook releases.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	store, ok := d.DataStore.(ReleaseStore)
	if !ok {
		return errors.New("webhook retention requires a datastore with webhook releases")
	}
	m.store, m.mx = store, d.Metrics
	if d.Log != nil {
		m.log = d.Log.With(filament.Field{Key: "component", Value: "webhookretention"})
	}
	return nil
}

// Start runs the sweep until ctx ends.
func (m *Module) Start(ctx context.Context) {
	go func() {
		defer m.Close()
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			if err := m.Sweep(ctx); err != nil && ctx.Err() == nil && m.log != nil {
				m.log.Error("webhook retention sweep failed", err,
					filament.Field{Key: "event.name", Value: "webhook_retention.sweep.failed"})
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Sweep purges once. The inbox is attached on first use, since the server
// creates it and may start after the control plane.
func (m *Module) Sweep(ctx context.Context) error {
	if m.inbox == nil {
		i, closeInbox, err := m.attach(ctx)
		if err != nil {
			return fmt.Errorf("attach inbox: %w", err)
		}
		m.inbox, m.close = i, closeInbox
	}
	releases, err := m.store.WebhookReleases(ctx, m.inbox.StreamName(), m.inbox.Incarnation())
	if err != nil {
		return err
	}
	now := m.now()
	cutoff := now.Add(-m.retention)
	result, err := m.inbox.Purge(ctx, inbox.Policy{Cutoff: cutoff, HoldCutoff: now.Add(-m.hold)}, releases)
	if err != nil {
		// A recreated stream needs a fresh attachment.
		m.Close()
		return err
	}
	m.report(now, result)
	// Every delivery of a source deleted before the cutoff is now purged.
	if _, err := m.store.DeleteWebhookEndpointsOfDeletedSources(ctx, cutoff.Add(-receiverCacheLag)); err != nil {
		return fmt.Errorf("forget deleted webhook endpoints: %w", err)
	}
	if m.mx != nil {
		if messages, bytes, err := m.inbox.Usage(ctx); err == nil {
			m.mx.Gauge("filament_webhook_inbox_messages").Set(float64(messages))
			m.mx.Gauge("filament_webhook_inbox_bytes").Set(float64(bytes))
		}
	}
	return nil
}

// report publishes the sweep's metrics and warns about deliveries that were
// dropped or are waiting longer than the retention period.
func (m *Module) report(now time.Time, result inbox.Result) {
	var waiting time.Duration
	if !result.Oldest.IsZero() {
		waiting = now.Sub(result.Oldest)
	}
	if m.mx != nil {
		m.mx.Counter("filament_webhook_purged_total").Add(float64(result.Purged))
		m.mx.Counter("filament_webhook_expired_total").Add(float64(result.Expired))
		m.mx.Gauge("filament_webhook_oldest_unreleased_seconds").Set(waiting.Seconds())
	}
	if m.log == nil {
		return
	}
	for _, endpoint := range result.ExpiredEndpoints {
		m.log.Warn("webhook deliveries dropped after the hold period",
			filament.Field{Key: "event.name", Value: "webhook_retention.deliveries.expired"},
			filament.Field{Key: "tenant_id", Value: endpoint.Tenant},
			filament.Field{Key: "source_connection_id", Value: endpoint.Endpoint},
			filament.Field{Key: "hold_hours", Value: int(m.hold.Hours())})
	}
	if waiting > m.retention {
		m.log.Warn("webhook deliveries are waiting for a pipeline",
			filament.Field{Key: "event.name", Value: "webhook_retention.deliveries.waiting"},
			filament.Field{Key: "tenant_id", Value: result.OldestEndpoint.Tenant},
			filament.Field{Key: "source_connection_id", Value: result.OldestEndpoint.Endpoint},
			filament.Field{Key: "oldest_seconds", Value: int(waiting.Seconds())},
			filament.Field{Key: "hold_hours", Value: int(m.hold.Hours())})
	}
}

// Close releases the inbox connection.
func (m *Module) Close() {
	if m.close != nil {
		m.close()
	}
	m.inbox, m.close = nil, nil
}
