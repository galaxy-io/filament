package source

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// OpenStream validates the conservative consumer contract before delivering rows.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if opts.CheckAuthority == nil {
		return nil, errors.New("nats: coordinator authority check required")
	}
	if s.js == nil || len(opts.Resources) != 1 || opts.Resources[0] != s.stream {
		return nil, errors.New("nats: select exactly the configured stream")
	}
	if err := opts.Attempt.Validate(); err != nil {
		return nil, err
	}
	info, err := s.js.StreamInfo(s.stream, nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	ci, err := s.js.ConsumerInfo(s.stream, s.consumer, nats.Context(ctx))
	if err != nil {
		return nil, err
	}
	if err := validateConsumer(ci.Config, s.consumer); err != nil {
		return nil, err
	}
	c := ci.Config
	// Length-prefix components to avoid identity collisions with separators.
	incarnation := fmt.Sprintf("%d:%s%d:%s%s", len(s.identity), s.identity, len(s.stream), s.stream, info.Created.UTC().Format(time.RFC3339Nano))
	domain := filament.DomainKey{Domain: s.stream, Incarnation: incarnation}
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
	if ci.AckFloor.Stream > committed {
		return nil, errors.New("nats: consumer acknowledged beyond certified progress")
	}
	if info.State.FirstSeq > committed+1 && info.State.Msgs > 0 {
		return nil, errors.New("nats: retained input no longer covers resume position")
	}
	sub, err := s.js.PullSubscribe("", s.consumer, nats.Bind(s.stream, s.consumer))
	if err != nil {
		return nil, err
	}
	session := &session{source: s, checkAuthority: opts.CheckAuthority, sub: sub, domain: domain, created: info.Created, consumerCreated: ci.Created, committed: committed, codecs: r}
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
