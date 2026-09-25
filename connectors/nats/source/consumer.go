package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/nats-io/nats.go/jetstream"

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

// ensureConsumer binds the existing consumer, creating a managed one when absent.
func (b consumerBinding) ensureConsumer(ctx context.Context, stream jetstream.Stream, committed uint64) (jetstream.Consumer, error) {
	consumer, err := stream.Consumer(ctx, b.consumer)
	if err == nil {
		return consumer, nil
	}
	if !b.managed || !errors.Is(err, jetstream.ErrConsumerNotFound) {
		return nil, err
	}
	cfg := jetstream.ConsumerConfig{Durable: b.consumer, Description: "Filament managed " + b.consumer, AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxAckPending: 1, AckWait: 30 * time.Second, ReplayPolicy: jetstream.ReplayInstantPolicy}
	if len(b.filters) == 1 {
		cfg.FilterSubject = b.filters[0]
	} else {
		cfg.FilterSubjects = b.filters
	}
	if committed > 0 {
		cfg.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		cfg.OptStartSeq = committed + 1
		if cfg.OptStartSeq == 0 {
			return nil, errors.New("nats: exhausted stream sequence")
		}
	}
	return stream.CreateConsumer(ctx, cfg)
}

func (b consumerBinding) validateConsumer(c jetstream.ConsumerConfig) error {
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
	if c.DeliverPolicy != jetstream.DeliverAllPolicy && c.DeliverPolicy != jetstream.DeliverByStartSequencePolicy {
		return errors.New("nats: incompatible managed delivery policy")
	}
	// Reuse the safety checks without treating intentional filters as gaps.
	c.FilterSubject = ""
	c.FilterSubjects = nil
	c.DeliverPolicy = jetstream.DeliverAllPolicy
	return validateExistingConsumer(c, b.consumer)
}

func validateExistingConsumer(c jetstream.ConsumerConfig, name string) error {
	if c.Durable != name || c.DeliverSubject != "" || c.AckPolicy != jetstream.AckExplicitPolicy || c.MaxAckPending != 1 || c.DeliverPolicy != jetstream.DeliverAllPolicy || c.MaxDeliver > 0 || c.HeadersOnly || c.FilterSubject != "" || len(c.FilterSubjects) > 0 || len(c.BackOff) > 0 || c.AckWait < time.Second {
		return errors.New("nats: require unfiltered durable pull consumer, deliver-all, explicit ack, unlimited delivery, MaxAckPending=1, AckWait>=1s")
	}
	return nil
}

// initialFloor is captured only for managed consumers with no certified progress.
// It is not a checkpoint: any actual acknowledgement ahead of committed still
// fails, even if retention has since removed the acknowledged message.
func validateConsumerProgress(info *jetstream.ConsumerInfo, committed, initialFloor uint64) error {
	if info.AckFloor.Stream <= committed {
		return nil
	}
	if committed == 0 && info.AckFloor.Consumer == 0 && info.AckFloor.Stream <= initialFloor {
		return nil
	}
	return errors.New("nats: consumer acknowledged beyond certified progress")
}
