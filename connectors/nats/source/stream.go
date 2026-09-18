package source

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// OpenStream binds each selected resource to its own dedicated consumer.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if err := opts.Attempt.Validate(); err != nil {
		return nil, err
	}
	if s.js == nil || opts.CheckAuthority == nil || len(opts.Resources) == 0 {
		return nil, errors.New("nats: configured source, authority check and selected resources required")
	}
	if len(s.bindings) == 0 {
		return s.openSubjects(ctx, opts)
	}
	bindings, err := selectStreams(s.bindings, opts.Resources)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, b := range bindings {
		selected[b.Stream] = true
	}
	for domain := range opts.CommittedPositions {
		if !selected[domain.Domain] {
			return nil, filament.ErrPositionIncomparable
		}
	}
	multi := &multiSession{}
	for _, binding := range bindings {
		childSource := *s
		childSource.stream, childSource.consumer = binding.Stream, binding.Consumer
		childSource.resource = binding.Stream
		childOpts := opts
		childOpts.Resources = []string{binding.Stream}
		childOpts.CommittedPositions = filament.DomainPositions{}
		for domain, position := range opts.CommittedPositions {
			if domain.Domain == binding.Stream {
				childOpts.CommittedPositions[domain] = position
			}
		}
		child, err := childSource.openSingleStream(ctx, childOpts)
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			return nil, errors.Join(err, multi.Close(cleanup))
		}
		multi.children = append(multi.children, child)
	}
	if len(multi.children) == 1 {
		return multi.children[0], nil
	}
	multi.queued = make([]*nats.Msg, len(multi.children))
	return multi, nil
}

// OpenStream validates the conservative consumer contract before delivering rows.
func (s *Source) openSingleStream(ctx context.Context, opts filament.StreamOpenOpts) (*session, error) {
	if opts.CheckAuthority == nil {
		return nil, errors.New("nats: coordinator authority check required")
	}
	if s.js == nil || len(opts.Resources) != 1 || opts.Resources[0] != s.stream {
		return nil, errors.New("nats: select exactly the configured stream")
	}
	session := &session{source: s}
	lifecycle, err := stream.NewSourceLifecycle(opts.Attempt, func(ctx context.Context) error {
		if err := session.authority(ctx); err != nil {
			return err
		}
		return opts.CheckAuthority(ctx)
	})
	if err != nil {
		return nil, err
	}
	session.lifecycle = lifecycle
	info, err := s.js.StreamInfo(s.stream, nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	// Length-prefix components to avoid identity collisions with separators.
	incarnation := fmt.Sprintf("%d:%s%d:%s%s", len(s.identity), s.identity, len(s.stream), s.stream, info.Created.UTC().Format(time.RFC3339Nano))
	domain := filament.DomainKey{Domain: s.stream, Incarnation: incarnation}
	if s.managed {
		domain = managedDomain(s.identity, s.resource, info)
	}
	r := &streamkit.Registry{}
	if err := RegisterCodec(r); err != nil {
		return nil, err
	}
	var committed uint64
	for d, p := range opts.CommittedPositions {
		if d != domain {
			return nil, filament.ErrPositionIncomparable
		}
		canonical, err := rowmodel.CanonicalPosition(r, p)
		if err != nil || canonical.Codec != PositionCodec || canonical.Version != 0 {
			return nil, errors.Join(filament.ErrPositionIncomparable, err)
		}
		committed, err = strconv.ParseUint(string(canonical.Value), 10, 64)
		if err != nil {
			return nil, err
		}
	}
	if err := opts.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	ci, err := s.ensureConsumer(ctx, committed)
	if err != nil {
		return nil, err
	}
	if err := s.validateConsumer(ci.Config); err != nil {
		return nil, err
	}
	c := ci.Config
	if s.managed && c.DeliverPolicy == nats.DeliverByStartSequencePolicy && (committed == ^uint64(0) || c.OptStartSeq > committed+1) {
		return nil, errors.New("nats: consumer starts beyond certified progress")
	}
	if ci.AckFloor.Stream > committed {
		return nil, errors.New("nats: consumer acknowledged beyond certified progress")
	}
	if (!s.managed || committed > 0) && info.State.FirstSeq > committed+1 && info.State.Msgs > 0 {
		return nil, errors.New("nats: retained input no longer covers resume position")
	}
	sub, err := s.js.PullSubscribe(ci.Config.FilterSubject, s.consumer, nats.Bind(s.stream, s.consumer))
	if err != nil {
		return nil, err
	}
	session := &session{source: s, checkAuthority: opts.CheckAuthority, sub: sub, domain: domain, created: info.Created, consumerCreated: ci.Created, committed: committed, codecs: r}
	session.scanFloor = committed
	if s.managed && committed == 0 && info.State.FirstSeq > 0 {
		session.scanFloor = info.State.FirstSeq - 1
	}
	hb, err := streamkit.StartHeartbeat(ctx, c.AckWait/3, func(ctx context.Context) error {
		session.mu.Lock()
		defer session.mu.Unlock()
		if session.pending != nil {
			return session.pending.InProgress(nats.Context(ctx))
		}
		return nil
	})
	if err != nil {
		_ = sub.Unsubscribe()
		return nil, err
	}
	session.hb = hb
	return session, nil
}

func validateConsumer(c nats.ConsumerConfig, name string) error {
	if c.Durable != name || c.DeliverSubject != "" || c.AckPolicy != nats.AckExplicitPolicy || c.MaxAckPending != 1 || c.DeliverPolicy != nats.DeliverAllPolicy || c.MaxDeliver > 0 || c.HeadersOnly || c.FilterSubject != "" || len(c.FilterSubjects) > 0 || len(c.BackOff) > 0 || c.AckWait < time.Second {
		return errors.New("nats: require unfiltered durable pull consumer, deliver-all, explicit ack, unlimited delivery, MaxAckPending=1, AckWait>=1s")
	}
	return nil
}
