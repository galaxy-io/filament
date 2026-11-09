package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	readerPrefetchMessages = 512
	readerPrefetchBytes    = 8 << 20
)

// Reader reads one endpoint's deliveries in stream order from a position the
// caller has already certified. It keeps no durable state and never
// acknowledges: certified progress lives with the caller, and retention, not
// acknowledgement, removes messages.
//
// One ephemeral consumer prefetches in the background, bounded by message
// count and bytes, so Next usually returns from a local buffer.
type Reader struct {
	messages jetstream.MessagesContext
	tenant   string
	endpoint string
}

// Reader opens an ordered, ephemeral consumer after certified. The expected
// incarnation is the stream creation time from certified progress, and a zero
// value is allowed only before the first certificate. Close the reader to
// release the consumer.
func (i *Inbox) Reader(ctx context.Context, tenant, endpoint string, certified uint64, expectedIncarnation time.Time) (*Reader, error) {
	if i == nil || tenant == "" || endpoint == "" || certified == ^uint64(0) {
		return nil, ErrInvalid
	}
	if !expectedIncarnation.IsZero() && !expectedIncarnation.Equal(i.incarnation) {
		return nil, errors.New("inbox: stream incarnation changed")
	}
	cfg := jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{endpointFilter(tenant, endpoint)},
		DeliverPolicy:  jetstream.DeliverAllPolicy,
	}
	if certified > 0 {
		cfg.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		cfg.OptStartSeq = certified + 1
	}
	consumer, err := i.stream.OrderedConsumer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("inbox: open reader: %w", err)
	}
	// The byte budget must hold at least one message of the largest size.
	budget := max(readerPrefetchBytes, 2*int(i.maxMsgSize))
	messages, err := consumer.Messages(jetstream.PullMaxMessagesWithBytesLimit(readerPrefetchMessages, budget))
	if err != nil {
		return nil, fmt.Errorf("inbox: start reader: %w", err)
	}
	return &Reader{messages: messages, tenant: tenant, endpoint: endpoint}, nil
}

// Next returns the next delivery and its stream sequence. It blocks until a
// delivery arrives or ctx ends.
func (r *Reader) Next(ctx context.Context) (Delivery, uint64, error) {
	if r == nil {
		return Delivery{}, 0, ErrInvalid
	}
	msg, err := r.messages.Next(jetstream.NextContext(ctx))
	if err != nil {
		return Delivery{}, 0, err
	}
	meta, err := msg.Metadata()
	if err != nil {
		return Delivery{}, 0, err
	}
	d, err := decode(msg.Subject(), msg.Headers(), msg.Data())
	if err != nil {
		return Delivery{}, 0, err
	}
	if d.Tenant != r.tenant || d.Endpoint != r.endpoint {
		return Delivery{}, 0, fmt.Errorf("inbox: %w: delivery belongs to another endpoint", ErrInvalid)
	}
	return d, meta.Sequence.Stream, nil
}

// Close stops prefetching and releases the consumer.
func (r *Reader) Close() {
	if r != nil {
		r.messages.Stop()
	}
}
