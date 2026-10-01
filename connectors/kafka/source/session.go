package source

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/streamkit"
)

type consumer interface {
	PollRecords(context.Context, int) kgo.Fetches
	Close()
}
type session struct {
	consumer   consumer
	metadata   *kgo.Client
	lifecycle  stream.SourceLifecycle
	topics     []string
	identity   string
	partitions map[partitionKey]*partitionState
	writers    map[string]arrowbatch.RowWriter
	columns    map[string]*streamkit.MessageColumns
	projectors map[string]*streamkit.Projector
	codecs     filament.CodecResolver
	validate   func(context.Context) error
	closeOnce  sync.Once
	closeDone  chan struct{}
	closed     bool
	closeErr   error
}

// OpenStream opens a fixed set of partitions from certified progress.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if s.client == nil || opts.CheckAuthority == nil || opts.SourceConnectionID == "" {
		return nil, errors.New("kafka: configured source, stable identity and authority required")
	}
	if err := opts.Attempt.Validate(); err != nil {
		return nil, err
	}
	if err := opts.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	selected, err := topics(opts.Resources)
	if err != nil {
		return nil, err
	}
	meta, err := client.Metadata(ctx, s.client, selected)
	if err != nil {
		return nil, err
	}
	ds, err := domains(meta, opts.SourceConnectionID)
	if err != nil {
		return nil, err
	}
	starts, err := listOffsets(ctx, s.client, ds, -2)
	if err != nil {
		return nil, err
	}
	ends, err := listOffsets(ctx, s.client, ds, -1)
	if err != nil {
		return nil, err
	}
	known := map[filament.DomainKey]bool{}
	for _, d := range ds {
		known[d] = true
	}
	for d := range opts.CommittedPositions {
		if !known[d] {
			return nil, filament.ErrPositionIncomparable
		}
	}
	ss := &session{metadata: s.client, topics: selected, identity: opts.SourceConnectionID, partitions: map[partitionKey]*partitionState{}, writers: map[string]arrowbatch.RowWriter{}, projectors: map[string]*streamkit.Projector{}, codecs: s}
	assigned := map[string]map[int32]kgo.Offset{}
	for k, d := range ds {
		next := starts[k]
		if s.start == "latest" {
			next = ends[k]
		}
		p, exists := opts.CommittedPositions[d]
		if exists {
			next, err = offset(p)
			if err != nil {
				return nil, err
			}
		}
		if next < starts[k] || next > ends[k] {
			return nil, errors.New("kafka: certified offset outside retained range")
		}
		ss.partitions[k] = &partitionState{domain: d, next: next, initial: !exists}
		if assigned[k.topic] == nil {
			assigned[k.topic] = map[int32]kgo.Offset{}
		}
		assigned[k.topic][k.partition] = kgo.NewOffset().At(next)
	}
	// Direct assignment avoids broker-owned commits and consumer group rebalances.
	// Fetch limits bound normal prefetch; Kafka can exceed its byte limit for a
	// single oversized record, so worker budgets must also include broker limits.
	clientOpts := append([]kgo.Opt{}, s.options...)
	clientOpts = append(clientOpts, kgo.ConsumePartitions(assigned), kgo.ConsumeResetOffset(kgo.NoResetOffset()), kgo.FetchIsolationLevel(kgo.ReadCommitted()), kgo.FetchMaxBytes(4<<20), kgo.FetchMaxPartitionBytes(1<<20), kgo.MaxConcurrentFetches(1))
	c, err := kgo.NewClient(clientOpts...)
	if err != nil {
		return nil, err
	}
	ss.consumer = c
	ss.validate = ss.validateMetadata
	ss.lifecycle, err = stream.NewSourceLifecycle(opts.Attempt, func(ctx context.Context) error {
		if err := opts.CheckAuthority(ctx); err != nil {
			return err
		}
		return ss.validate(ctx)
	})
	if err != nil {
		c.Close()
		return nil, err
	}
	return ss, nil
}

// Read emits at most one message per epoch initially, like the NATS reference.
// Initial frontiers are certified before reading data, including idle partitions.
func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (coverage filament.Coverage, result error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return coverage, err
	}
	if b.MaxWait <= 0 {
		return coverage, errors.New("kafka: positive MaxWait required")
	}
	defer func() {
		if result != nil {
			s.lifecycle.Fail(result)
		}
	}()
	initial := filament.DomainPositions{}
	for _, p := range s.partitions {
		if p.initial {
			initial[p.domain] = position(p.next)
		}
	}
	if len(initial) > 0 {
		keys := make([]filament.DomainKey, 0, len(initial))
		for d := range initial {
			keys = append(keys, d)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].Domain < keys[j].Domain })
		for _, d := range keys {
			if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: d, Position: initial[d]}); err != nil {
				return coverage, err
			}
		}
		coverage.Positions = initial
		return coverage, s.lifecycle.MarkRead(coverage)
	}
	readCtx, cancel := context.WithTimeout(ctx, b.MaxWait)
	defer cancel()
	fetched := s.consumer.PollRecords(readCtx, 1)
	if err := ctx.Err(); err != nil {
		return coverage, err
	}
	if fetched.NumRecords() == 0 && errors.Is(fetched.Err(), context.DeadlineExceeded) {
		return coverage, nil
	}
	if err := fetched.Err(); err != nil {
		return coverage, err
	}
	records := fetched.Records()
	if len(records) == 0 {
		return coverage, nil
	}
	if err := s.lifecycle.CheckAuthority(ctx); err != nil {
		return coverage, err
	}
	return s.readRecord(ctx, out, records[0])
}

func (s *session) readRecord(ctx context.Context, out filament.StreamRecordSink, r *kgo.Record) (coverage filament.Coverage, result error) {
	key := partitionKey{r.Topic, r.Partition}
	p, ok := s.partitions[key]
	if !ok || r.Offset < p.next || r.Offset == math.MaxInt64 {
		return coverage, errors.New("kafka: invalid record position or partition")
	}
	writer := s.writers[r.Topic]
	if writer == nil {
		columns, schema, err := streamkit.NewMessageColumns(messageBaseSchema(r.Topic), r.Value)
		if err != nil {
			return coverage, err
		}
		writer, err = out.Builder(r.Topic, 0, schema)
		if err != nil {
			return coverage, err
		}
		s.writers[r.Topic] = writer
		s.projectors[r.Topic] = streamkit.NewEventMetadataProjector(writer, s.codecs)
		if s.columns == nil {
			s.columns = map[string]*streamkit.MessageColumns{}
		}
		s.columns[r.Topic] = columns
	}
	if err := appendRecord(writer, s.projectors[r.Topic], s.columns[r.Topic], p.domain, r); err != nil {
		return coverage, err
	}
	next := position(r.Offset + 1)
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: p.domain, Position: next}); err != nil {
		return coverage, err
	}
	coverage.Positions = filament.DomainPositions{p.domain: next}
	return coverage, s.lifecycle.MarkRead(coverage)
}

// Acknowledge validates certified coverage. Direct assignment has no external
// acknowledgment; only Filament's durable certificate determines future resume.
func (s *session) Acknowledge(ctx context.Context, c filament.Coverage) error {
	if err := s.lifecycle.CheckAcknowledge(ctx, c); err != nil {
		return err
	}
	for _, p := range s.partitions {
		if v, ok := c.Positions[p.domain]; ok {
			n, err := offset(v)
			if err != nil {
				return err
			}
			p.next = n
			p.initial = false
		}
	}
	return s.lifecycle.MarkAcknowledged(c)
}

func (s *session) Close(ctx context.Context) error {
	if s.closed {
		return s.closeErr
	}
	s.closeOnce.Do(func() {
		s.closeDone = make(chan struct{})
		go func() { defer close(s.closeDone); s.consumer.Close() }()
	})
	select {
	case <-s.closeDone:
		s.closed = true
		s.closeErr = ctx.Err()
		s.lifecycle.MarkClosed(s.closeErr)
		return s.closeErr
	case <-ctx.Done():
		s.lifecycle.Fail(ctx.Err())
		return ctx.Err()
	}
}
