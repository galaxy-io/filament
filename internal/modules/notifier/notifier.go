// Package notifier delivers pipeline notifications from ingestion events.
package notifier

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	notification "github.com/galaxy-io/filament/internal/notifier"
	"github.com/galaxy-io/filament/module"
)

const (
	operationTimeout = 18 * time.Second
	reportTimeout    = 7 * time.Second
	progressInterval = 10 * time.Second
	maxConcurrent    = 8
)

// Module matches current pipeline rules and reports delivery attempts.
type Module struct {
	ds      filament.DataStore
	store   notification.Store
	bus     eventbus.Bus
	secrets filament.Secrets
	log     filament.Logger
	senders map[notification.NotificationType]notification.Sender
}

var _ module.Module = (*Module)(nil)

// New returns an unmounted notifier with its supported senders.
func New(senders map[notification.NotificationType]notification.Sender) *Module {
	return &Module{senders: maps.Clone(senders)}
}

// Name identifies this module.
func (m *Module) Name() string { return "notifier" }

// Subscriptions declares a live-tail consumer that resumes pending events on restart.
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{{
		Pattern: events.AllPattern(), Durable: "notifier", Replay: false,
		MaxInFlight: 1, Handler: m.onFact,
	}}
}

// Mount captures the providers this module uses, without starting delivery.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	store, ok := d.DataStore.(notification.Store)
	if !ok {
		return errors.New("datastore does not support notifiers")
	}
	if d.Bus == nil {
		return errors.New("notifier requires an event bus")
	}
	m.ds = d.DataStore
	m.store = store
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
	if notification.IsLifecycleEvent(f.Name) {
		return nil
	}
	if _, ok := events.Lookup(f.Name); !ok || f.Tenant.Valid() != nil || f.Run.Valid() != nil {
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
	rules, request, err := m.rulesFor(lookupCtx, f)
	cancelLookup()
	if err != nil || len(rules) == 0 {
		return err
	}
	frame, err := events.Marshal(f)
	if err != nil {
		return errors.New("notifier: could not encode trigger event")
	}
	trigger := notification.Notification{
		Tenant: f.Tenant, Run: f.Run, Resource: f.Resource,
		PipelineID: request.PipelineID, PipelineVersionID: request.PipelineVersionID,
		TriggerType: f.Name, TriggerSubject: msg.Subject(), TriggerStreamSequence: msg.Seq(), Event: frame,
	}
	return m.deliverAll(ctx, trigger, rules)
}

func (m *Module) rulesFor(ctx context.Context, f events.Fact) ([]notification.Notifier, filament.RunRequest, error) {
	run, err := m.ds.LoadRun(ctx, f.Tenant, f.Run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, filament.RunRequest{}, nil
	}
	if err != nil {
		return nil, filament.RunRequest{}, errors.New("notifier: could not load run")
	}
	if run.Request.PipelineID == "" {
		return nil, run.Request, nil
	}
	pipeline, err := m.ds.LoadPipeline(ctx, f.Tenant, run.Request.PipelineID)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, run.Request, nil
	}
	if err != nil {
		return nil, run.Request, errors.New("notifier: could not load pipeline")
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, run.Request, nil
	}
	rules, err := m.store.ListNotifiers(ctx, notification.Filter{Tenant: f.Tenant, PipelineID: run.Request.PipelineID})
	if err != nil {
		return nil, run.Request, errors.New("notifier: could not list pipeline rules")
	}
	selected := make([]notification.Notifier, 0, len(rules))
	for _, n := range rules {
		if n.Tenant != f.Tenant || n.PipelineID != run.Request.PipelineID {
			return nil, run.Request, errors.New("notifier: rule does not belong to this pipeline")
		}
		if notification.Matches(n, f.Name, f.Resource) {
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
