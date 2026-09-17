// Package notifier delivers pipeline notifications from ingestion events.
package notifier

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	notification "github.com/galaxy-io/filament/internal/notifier"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
	"github.com/galaxy-io/filament/module"
)

const (
	operationTimeout = 18 * time.Second
	reportTimeout    = 7 * time.Second
	progressInterval = 10 * time.Second
	maxConcurrent    = 8

	// Retries double from backoffBase up to backoffCap, so the whole window
	// for one rule stays under ten minutes.
	maxAttempts = 10
	backoffBase = time.Second
	backoffCap  = 2 * time.Minute

	// maxInFlight is both the unacked cap and the handler worker count per
	// exported kind, so one slow endpoint holds only its own facts.
	maxInFlight = 16
)

// Module matches current pipeline rules and reports delivery attempts.
type Module struct {
	ds      filament.DataStore
	bus     eventbus.Bus
	secrets filament.Secrets
	log     filament.Logger
	senders map[ingestionv1.NotificationType]notification.Sender
}

var _ module.Module = (*Module)(nil)

// New returns an unmounted notifier with its supported senders.
func New() *Module {
	return &Module{senders: map[ingestionv1.NotificationType]notification.Sender{
		ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK: webhook.New(nil),
	}}
}

// Name identifies this module.
func (m *Module) Name() string { return "notifier" }

// Subscriptions declares one live-tail consumer per exported event kind.
func (m *Module) Subscriptions() []host.Subscription {
	exported := notification.Exported()
	subs := make([]host.Subscription, 0, len(exported))
	for _, d := range exported {
		subs = append(subs, host.Subscription{
			Pattern: d.Pattern(), Durable: "notifier-" + d.Entity + "-" + d.Event, Replay: false,
			MaxInFlight: maxInFlight, Handler: m.onFact,
		})
	}
	return subs
}

// Mount captures the providers this module uses, without starting delivery.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	if d.DataStore == nil {
		return errors.New("notifier requires a datastore")
	}
	if d.Bus == nil {
		return errors.New("notifier requires an event bus")
	}
	m.ds = d.DataStore
	m.bus = d.Bus
	m.secrets = d.Secrets
	if d.Log != nil {
		m.log = d.Log.With(filament.Field{Key: "component", Value: "notifier"})
	}
	return nil
}

// onFact retries the source event only for lookup failures or interrupted processing.
func (m *Module) onFact(ctx context.Context, msg eventbus.Message) error {
	f, err := events.Decode(msg)
	if err != nil {
		m.ignored("decode_failed", msg.Seq())
		return nil
	}
	event, err := notification.ParseEventName(f.Name)
	if err != nil || f.Tenant.Valid() != nil || f.Run.Valid() != nil {
		m.ignored("invalid_event", msg.Seq())
		return nil
	}
	if msg.Seq() == 0 {
		return errors.New("notifier: trigger requires a stream sequence")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := m.keepAlive(ctx, cancel, msg)
	defer stop()
	lookupCtx, cancelLookup := context.WithTimeout(ctx, operationTimeout)
	rules, request, err := m.rulesFor(lookupCtx, f, event)
	cancelLookup()
	if err != nil || len(rules) == 0 {
		return err
	}
	frame, err := notification.Project(f)
	if err != nil {
		return err
	}
	trigger := notification.Notification{
		Tenant: f.Tenant, Run: f.Run, Resource: f.Resource,
		PipelineID: request.PipelineID, PipelineVersionID: request.PipelineVersionID,
		TriggerType: f.Name, TriggerEvent: event, TriggerSubject: msg.Subject(), TriggerStreamSequence: msg.Seq(), Event: frame,
	}
	return m.deliverAll(ctx, trigger, rules)
}

func (m *Module) rulesFor(ctx context.Context, f events.Fact, event ingestionv1.NotifierEvent) ([]*ingestionv1.Notifier, filament.RunRequest, error) {
	run, err := m.ds.LoadRun(ctx, f.Tenant, f.Run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, filament.RunRequest{}, nil
	}
	if err != nil {
		return nil, filament.RunRequest{}, fmt.Errorf("notifier: load run: %w", err)
	}
	if run.Request.PipelineID == "" {
		return nil, run.Request, nil
	}
	pipeline, err := m.ds.LoadPipeline(ctx, f.Tenant, run.Request.PipelineID)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, run.Request, nil
	}
	if err != nil {
		return nil, run.Request, fmt.Errorf("notifier: load pipeline: %w", err)
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, run.Request, nil
	}
	rules, err := m.ds.ListNotifiers(ctx, f.Tenant, run.Request.PipelineID, false)
	if err != nil {
		return nil, run.Request, fmt.Errorf("notifier: list pipeline rules: %w", err)
	}
	selected := make([]*ingestionv1.Notifier, 0, len(rules))
	for _, n := range rules {
		if filament.TenantID(n.GetTenantId()) != f.Tenant || n.GetPipelineId() != run.Request.PipelineID {
			return nil, run.Request, errors.New("notifier: rule does not belong to this pipeline")
		}
		if notification.Matches(n, event, f.Resource) {
			selected = append(selected, n)
		}
	}
	return selected, run.Request, nil
}

func (m *Module) ignored(reason string, sequence uint64) {
	if m.log != nil {
		m.log.Trace("notification event ignored",
			filament.Field{Key: "event.name", Value: "notifier.fact.ignored"},
			filament.Field{Key: "reason", Value: reason},
			filament.Field{Key: "stream_sequence", Value: sequence})
	}
}

// keepAlive prevents redelivery while a large batch is still being processed.
func (m *Module) keepAlive(ctx context.Context, cancel context.CancelFunc, msg eventbus.Message) func() {
	progress, ok := msg.(eventbus.ProgressReporter)
	if !ok {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			if err := progress.InProgress(); err != nil {
				if m.log != nil {
					m.log.Warn("notification acknowledgement extension failed",
						filament.Field{Key: "event.name", Value: "notifier.acknowledgement.failed"},
						filament.Field{Key: "stream_sequence", Value: msg.Seq()})
				}
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
