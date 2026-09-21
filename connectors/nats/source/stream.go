package source

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
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
	identity, err := sourceIdentity(s.identity, opts.SourceConnectionID)
	if err != nil {
		return nil, err
	}
	// Each session gets its own binding; reusing a configured source must not
	// retain an identity derived for a previous connection.
	bound := *s
	bound.identity = identity
	s = &bound
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
		consumer := consumerBinding{stream: binding.Stream, consumer: binding.Consumer, resource: binding.Stream}
		childOpts := opts
		childOpts.Resources = []string{binding.Stream}
		childOpts.CommittedPositions = filament.DomainPositions{}
		for domain, position := range opts.CommittedPositions {
			if domain.Domain == binding.Stream {
				childOpts.CommittedPositions[domain] = position
			}
		}
		child, err := s.openSingleStream(ctx, consumer, childOpts)
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			closeErr := multi.Close(cleanup)
			cancel()
			return nil, errors.Join(err, closeErr)
		}
		multi.children = append(multi.children, child)
	}
	if len(multi.children) == 1 {
		return multi.children[0], nil
	}
	multi.queued = make([]*nats.Msg, len(multi.children))
	return multi, nil
}

// openSingleStream validates the consumer contract before delivering rows.
func (s *Source) openSingleStream(ctx context.Context, binding consumerBinding, opts filament.StreamOpenOpts) (*session, error) {
	if opts.CheckAuthority == nil {
		return nil, errors.New("nats: coordinator authority check required")
	}
	if s.js == nil || len(opts.Resources) != 1 || opts.Resources[0] != binding.stream {
		return nil, errors.New("nats: select exactly the configured stream")
	}
	session := &session{js: s.js, binding: binding}
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
	info, err := s.js.StreamInfo(binding.stream, nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	domain := consumerDomain(s.identity, binding.stream, info)
	if binding.managed {
		domain = managedDomain(s.identity, binding.resource, info)
	}
	r := &streamkit.Registry{}
	if err := RegisterCodec(r); err != nil {
		return nil, err
	}
	committed, err := committedSequence(opts.CommittedPositions, domain, r)
	if err != nil {
		return nil, err
	}
	if err := opts.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	ci, err := binding.ensureConsumer(ctx, s.js, committed)
	if err != nil {
		return nil, err
	}
	if err := binding.validateConsumer(ci.Config); err != nil {
		return nil, err
	}
	c := ci.Config
	if binding.managed && c.DeliverPolicy == nats.DeliverByStartSequencePolicy && (committed == ^uint64(0) || c.OptStartSeq > committed+1) {
		return nil, errors.New("nats: consumer starts beyond certified progress")
	}
	if ci.AckFloor.Stream > committed {
		return nil, errors.New("nats: consumer acknowledged beyond certified progress")
	}
	if (!binding.managed || committed > 0) && info.State.FirstSeq > committed+1 && info.State.Msgs > 0 {
		return nil, errors.New("nats: retained input no longer covers resume position")
	}
	sub, err := s.js.PullSubscribe(ci.Config.FilterSubject, binding.consumer, nats.Bind(binding.stream, binding.consumer))
	if err != nil {
		return nil, err
	}
	session.sub = sub
	session.domain = domain
	session.created = info.Created
	session.consumerCreated = ci.Created
	session.committed = committed
	session.codecs = r
	session.scanFloor = committed
	if binding.managed && committed == 0 && info.State.FirstSeq > 0 {
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

// openSubjects assembles consumers for the resolved logical resources.
func (s *Source) openSubjects(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	targets, err := s.resolveSubjects(ctx, opts)
	if err != nil {
		return nil, err
	}
	multi := &multiSession{writers: map[string]arrowbatch.RowWriter{}}
	for _, target := range targets {
		childOpts := opts
		childOpts.Resources = []string{target.binding.stream}
		childOpts.CommittedPositions = filament.DomainPositions{}
		if p, ok := opts.CommittedPositions[target.domain]; ok {
			childOpts.CommittedPositions[target.domain] = p
		}
		child, err := s.openSingleStream(ctx, target.binding, childOpts)
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			closeErr := multi.Close(cleanup)
			cancel()
			return nil, errors.Join(err, closeErr)
		}
		multi.children = append(multi.children, child)
	}
	multi.queued = make([]*nats.Msg, len(multi.children))
	return multi, nil
}

func committedSequence(positions filament.DomainPositions, domain filament.DomainKey, r filament.CodecResolver) (uint64, error) {
	var committed uint64
	for d, p := range positions {
		if d != domain {
			return 0, filament.ErrPositionIncomparable
		}
		canonical, err := rowmodel.CanonicalPosition(r, p)
		if err != nil || canonical.Codec != PositionCodec || canonical.Version != 0 {
			return 0, errors.Join(filament.ErrPositionIncomparable, err)
		}
		committed, err = strconv.ParseUint(string(canonical.Value), 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return committed, nil
}
