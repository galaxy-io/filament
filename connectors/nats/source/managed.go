package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func managedDomain(identity, resource string, info *nats.StreamInfo) filament.DomainKey {
	return filament.DomainKey{Domain: fmt.Sprintf("%d:%s%s", len(resource), resource, info.Config.Name), Incarnation: fmt.Sprintf("%d:%s%s", len(identity), identity, info.Created.UTC().Format(time.RFC3339Nano))}
}
func managedConsumer(attempt filament.AttemptRef, resource, physical string) string {
	raw, _ := json.Marshal([]any{attempt.StreamID, attempt.Generation, resource, physical})
	sum := sha256.Sum256(raw)
	return "filament_" + hex.EncodeToString(sum[:20])
}

// openSubjects resolves logical resource patterns against all existing physical
// streams. Consumers are durable and retained across worker and run restarts.
func (s *Source) openSubjects(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	patterns := slices.Clone(opts.Resources)
	slices.Sort(patterns)
	for i, p := range patterns {
		if err := validateSubject(p); err != nil {
			return nil, err
		}
		if i > 0 && patterns[i-1] == p {
			return nil, fmt.Errorf("nats: duplicate resource %q", p)
		}
	}
	js, err := jetstream.New(s.conn)
	if err != nil {
		return nil, err
	}
	names := js.StreamNames(ctx)
	var physical []string
	for name := range names.Name() {
		physical = append(physical, name)
	}
	if err := names.Err(); err != nil {
		return nil, err
	}
	slices.Sort(physical)
	type target struct {
		source Source
		domain filament.DomainKey
	}
	var targets []target
	found := map[string]bool{}
	domains := map[filament.DomainKey]bool{}
	for _, name := range physical {
		info, err := s.js.StreamInfo(name, nats.Context(ctx))
		if err != nil {
			return nil, err
		}
		for _, pattern := range patterns {
			filters := subjectFilters(pattern, info.Config.Subjects)
			if len(filters) == 0 {
				continue
			}
			child := *s
			child.stream, child.resource = name, pattern
			child.consumer = managedConsumer(opts.Attempt, pattern, name)
			child.filters, child.managed = filters, true
			domain := managedDomain(s.identity, pattern, info)
			targets = append(targets, target{child, domain})
			domains[domain] = true
			found[pattern] = true
		}
	}
	for _, pattern := range patterns {
		if !found[pattern] {
			return nil, fmt.Errorf("nats: no JetStream stream stores subjects matching %q", pattern)
		}
	}
	// Check all saved incarnations before creating any consumer. Losing a backing
	// stream must not silently discard previously certified progress.
	for domain := range opts.CommittedPositions {
		if !domains[domain] {
			return nil, filament.ErrPositionIncomparable
		}
	}
	multi := &multiSession{writers: map[string]arrowbatch.RowWriter{}}
	for _, target := range targets {
		childOpts := opts
		childOpts.Resources = []string{target.source.stream}
		childOpts.CommittedPositions = filament.DomainPositions{}
		if p, ok := opts.CommittedPositions[target.domain]; ok {
			childOpts.CommittedPositions[target.domain] = p
		}
		child, err := target.source.openSingleStream(ctx, childOpts)
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			return nil, errors.Join(err, multi.Close(cleanup))
		}
		multi.children = append(multi.children, child)
	}
	multi.queued = make([]*nats.Msg, len(multi.children))
	return multi, nil
}

func (s *Source) ensureConsumer(ctx context.Context, committed uint64) (*nats.ConsumerInfo, error) {
	info, err := s.js.ConsumerInfo(s.stream, s.consumer, nats.Context(ctx))
	if err == nil {
		return info, nil
	}
	if !s.managed || !errors.Is(err, nats.ErrConsumerNotFound) {
		return nil, err
	}
	cfg := &nats.ConsumerConfig{Durable: s.consumer, Description: "Filament managed " + s.consumer, AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: 30 * time.Second, ReplayPolicy: nats.ReplayInstantPolicy}
	if len(s.filters) == 1 {
		cfg.FilterSubject = s.filters[0]
	} else {
		cfg.FilterSubjects = s.filters
	}
	if committed > 0 {
		cfg.DeliverPolicy = nats.DeliverByStartSequencePolicy
		cfg.OptStartSeq = committed + 1
		if cfg.OptStartSeq == 0 {
			return nil, errors.New("nats: exhausted stream sequence")
		}
	}
	return s.js.AddConsumer(s.stream, cfg, nats.Context(ctx))
}

func (s *Source) validateConsumer(c nats.ConsumerConfig) error {
	if !s.managed {
		return validateConsumer(c, s.consumer)
	}
	filters := slices.Clone(c.FilterSubjects)
	if c.FilterSubject != "" {
		filters = append(filters, c.FilterSubject)
	}
	slices.Sort(filters)
	if !slices.Equal(filters, s.filters) || c.Description != "Filament managed "+s.consumer {
		return errors.New("nats: managed consumer identity or filters changed")
	}
	if c.DeliverPolicy != nats.DeliverAllPolicy && c.DeliverPolicy != nats.DeliverByStartSequencePolicy {
		return errors.New("nats: incompatible managed delivery policy")
	}
	// Reuse the safety checks without treating intentional filters as gaps.
	c.FilterSubject = ""
	c.FilterSubjects = nil
	c.DeliverPolicy = nats.DeliverAllPolicy
	return validateConsumer(c, s.consumer)
}
