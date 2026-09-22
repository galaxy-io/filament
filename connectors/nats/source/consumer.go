package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

// consumerBinding describes one physical consumer serving a logical resource.
// It is fixed for the lifetime of a session; Source owns the shared connection.
type consumerBinding struct {
	stream, consumer, resource string
	filters                    []string
	managed                    bool
}

func managedConsumer(attempt filament.AttemptRef, resource, physical string) string {
	raw, _ := json.Marshal([]any{attempt.StreamID, attempt.Generation, resource, physical})
	sum := sha256.Sum256(raw)
	return "filament_" + hex.EncodeToString(sum[:20])
}

func (b consumerBinding) ensureConsumer(ctx context.Context, js nats.JetStreamContext, committed uint64) (*nats.ConsumerInfo, error) {
	info, err := js.ConsumerInfo(b.stream, b.consumer, nats.Context(ctx))
	if err == nil {
		return info, nil
	}
	if !b.managed || !errors.Is(err, nats.ErrConsumerNotFound) {
		return nil, err
	}
	cfg := &nats.ConsumerConfig{Durable: b.consumer, Description: "Filament managed " + b.consumer, AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: 30 * time.Second, ReplayPolicy: nats.ReplayInstantPolicy}
	if len(b.filters) == 1 {
		cfg.FilterSubject = b.filters[0]
	} else {
		cfg.FilterSubjects = b.filters
	}
	if committed > 0 {
		cfg.DeliverPolicy = nats.DeliverByStartSequencePolicy
		cfg.OptStartSeq = committed + 1
		if cfg.OptStartSeq == 0 {
			return nil, errors.New("nats: exhausted stream sequence")
		}
	}
	return js.AddConsumer(b.stream, cfg, nats.Context(ctx))
}

func (b consumerBinding) validateConsumer(c nats.ConsumerConfig) error {
	if !b.managed {
		return validateExistingConsumer(c, b.consumer)
	}
	filters := slices.Clone(c.FilterSubjects)
	if c.FilterSubject != "" {
		filters = append(filters, c.FilterSubject)
	}
	slices.Sort(filters)
	if !slices.Equal(filters, b.filters) || c.Description != "Filament managed "+b.consumer {
		return errors.New("nats: managed consumer identity or filters changed")
	}
	if c.DeliverPolicy != nats.DeliverAllPolicy && c.DeliverPolicy != nats.DeliverByStartSequencePolicy {
		return errors.New("nats: incompatible managed delivery policy")
	}
	// Reuse the safety checks without treating intentional filters as gaps.
	c.FilterSubject = ""
	c.FilterSubjects = nil
	c.DeliverPolicy = nats.DeliverAllPolicy
	return validateExistingConsumer(c, b.consumer)
}

func validateExistingConsumer(c nats.ConsumerConfig, name string) error {
	if c.Durable != name || c.DeliverSubject != "" || c.AckPolicy != nats.AckExplicitPolicy || c.MaxAckPending != 1 || c.DeliverPolicy != nats.DeliverAllPolicy || c.MaxDeliver > 0 || c.HeadersOnly || c.FilterSubject != "" || len(c.FilterSubjects) > 0 || len(c.BackOff) > 0 || c.AckWait < time.Second {
		return errors.New("nats: require unfiltered durable pull consumer, deliver-all, explicit ack, unlimited delivery, MaxAckPending=1, AckWait>=1s")
	}
	return nil
}
