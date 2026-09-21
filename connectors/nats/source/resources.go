package source

import (
	"context"
	"fmt"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type resourceTarget struct {
	binding consumerBinding
	domain  filament.DomainKey
}

// resolveSubjects maps logical patterns to physical streams and validates saved
// domains before any consumer is created.
func (s *Source) resolveSubjects(ctx context.Context, opts filament.StreamOpenOpts) ([]resourceTarget, error) {
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
	var targets []resourceTarget
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
			binding := consumerBinding{stream: name, resource: pattern, consumer: managedConsumer(opts.Attempt, pattern, name), filters: filters, managed: true}
			domain := managedDomain(s.identity, pattern, info)
			targets = append(targets, resourceTarget{binding, domain})
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
	return targets, nil
}
