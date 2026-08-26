package runner

import (
	"context"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

type controlSignal int32

const (
	controlNone controlSignal = iota
	controlPause
	controlCancel
)

// runControl tails commands addressed to one active run. The subscription is
// established before run.started is published, so once the API can observe a
// running run there is always a worker ready to receive its command.
type runControl struct {
	tenant filament.TenantID
	run    filament.RunID
	log    filament.Logger

	mu            sync.Mutex
	signal        controlSignal
	sealed        bool
	cancelExtract context.CancelFunc
	subs          []eventbus.Subscription
	wg            sync.WaitGroup
}

func newRunControl(
	ctx context.Context,
	bus eventbus.Bus,
	log filament.Logger,
	tenant filament.TenantID,
	run filament.RunID,
) (context.Context, *runControl, error) {
	if bus == nil {
		return nil, nil, fmt.Errorf("runner: run control requires an event bus")
	}
	extractCtx, cancelExtract := context.WithCancel(ctx)
	control := &runControl{tenant: tenant, run: run, log: log, cancelExtract: cancelExtract}
	commands := []struct {
		name    string
		subject string
		signal  controlSignal
	}{
		{events.RunPauseRequested.Name(), events.Subject(events.RunPauseRequested, tenant, run), controlPause},
		{events.RunCancelRequested.Name(), events.Subject(events.RunCancelRequested, tenant, run), controlCancel},
	}
	for _, command := range commands {
		sub, err := bus.Subscribe(command.subject, eventbus.SubOpts{})
		if err != nil {
			control.close()
			return nil, nil, fmt.Errorf("runner: subscribe to %s: %w", command.name, err)
		}
		control.subs = append(control.subs, sub)
		control.wg.Add(1)
		go control.pump(ctx, sub, command.name, command.signal)
	}
	return extractCtx, control, nil
}

func (c *runControl) pump(ctx context.Context, sub eventbus.Subscription, name string, signal controlSignal) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.C():
			if !ok {
				return
			}
			fact, err := events.Decode(msg)
			if err == nil && fact.Name == name && fact.Tenant == c.tenant && fact.Run == c.run && c.request(signal) && c.log != nil {
				c.log.Info("runner: control requested",
					filament.Field{Key: "run", Value: string(c.run)},
					filament.Field{Key: "control", Value: signal.String()},
				)
			}
			_ = msg.Ack()
		}
	}
}

func (c *runControl) request(signal controlSignal) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return false
	}
	previous := c.signal
	switch signal {
	case controlCancel:
		c.signal = controlCancel
	case controlPause:
		if c.signal == controlNone {
			c.signal = controlPause
		}
	}
	c.cancelExtract()
	return c.signal != previous
}

func (s controlSignal) String() string {
	switch s {
	case controlPause:
		return "pause"
	case controlCancel:
		return "cancel"
	default:
		return "none"
	}
}

func (c *runControl) requested() controlSignal {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.signal
}

func (c *runControl) seal() controlSignal {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sealed = true
	return c.signal
}

func (c *runControl) close() {
	c.cancelExtract()
	for _, sub := range c.subs {
		_ = sub.Close()
	}
	c.wg.Wait()
}
