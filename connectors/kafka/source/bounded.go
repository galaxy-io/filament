package source

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

type boundedPartition struct {
	key        partitionKey
	domain     filament.DomainKey
	start, end int64
}

type boundedTopic struct {
	partitions []boundedPartition
	columns    *streamkit.MessageColumns
	schema     rowmodel.Schema
}

// PlanResources captures a finite read_committed window before destination DDL.
// Full reads always start at the retained beginning; start_position only applies
// to continuous sessions. Plans live for this configured source/run, not across runs.
func (s *Source) PlanResources(ctx context.Context, resources, selectors []string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.client == nil {
		return nil, errors.New("kafka: source is not configured")
	}
	if len(selectors) != 0 {
		return nil, errors.New("kafka: resource selectors are not supported")
	}
	if len(resources) == 0 {
		resources = s.defaultTopics
	}
	if len(resources) == 0 {
		discovered, err := s.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			return nil, err
		}
		for _, resource := range discovered.Resources {
			resources = append(resources, resource.Name)
		}
	}
	selected, err := topics(resources)
	if err != nil {
		return nil, err
	}
	if s.bounded == nil {
		s.bounded = map[string]*boundedTopic{}
	}
	var missing []string
	for _, topic := range selected {
		if s.bounded[topic] == nil {
			missing = append(missing, topic)
		}
	}
	if len(missing) == 0 {
		return selected, nil
	}
	metadata, err := client.Metadata(ctx, s.client, missing)
	if err != nil {
		return nil, err
	}
	ds, err := domains(metadata, "bounded")
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
	plans := map[string]*boundedTopic{}
	for _, topic := range missing {
		plans[topic] = &boundedTopic{}
	}
	for key, domain := range ds {
		if starts[key] > ends[key] {
			return nil, errors.New("kafka: invalid bounded offset range")
		}
		plans[key.topic].partitions = append(plans[key.topic].partitions, boundedPartition{key, domain, starts[key], ends[key]})
	}
	for _, topic := range missing {
		plan := plans[topic]
		slices.SortFunc(plan.partitions, func(a, b boundedPartition) int { return int(a.key.partition - b.key.partition) })
		var payload []byte
		found := false
		for _, part := range plan.partitions {
			for next := part.start; next < part.end; {
				records, advanced, err := s.fetchBounded(ctx, part, next)
				if err != nil {
					return nil, err
				}
				next = advanced
				if len(records) > 0 {
					payload, found = records[0].Value, true
					break
				}
			}
			if found {
				break
			}
		}
		plan.columns, plan.schema, err = streamkit.NewMessageColumns(messageBaseSchema(topic), payload)
		if err != nil {
			return nil, err
		}
	}
	for topic, plan := range plans {
		s.bounded[topic] = plan
	}
	return selected, nil
}

// fetchBounded uses the client's record parser so control batches, aborted
// transactions and compacted batches advance progress without emitting rows.
// It never treats a timeout as end-of-data.
func (s *Source) fetchBounded(ctx context.Context, part boundedPartition, next int64) ([]*kgo.Record, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, next, err
	}
	metadata, err := client.Metadata(ctx, s.client, []string{part.key.topic})
	if err != nil {
		return nil, next, err
	}
	ds, err := domains(metadata, "bounded")
	if err != nil {
		return nil, next, err
	}
	if ds[part.key] != part.domain {
		return nil, next, filament.ErrPositionIncomparable
	}
	topic := metadata.Topics[0]
	var leader, epoch int32 = -1, -1
	for _, p := range topic.Partitions {
		if p.Partition == part.key.partition {
			leader, epoch = p.Leader, p.LeaderEpoch
		}
	}
	if leader < 0 {
		return nil, next, errors.New("kafka: bounded partition has no leader")
	}
	request := kmsg.NewPtrFetchRequest()
	request.IsolationLevel, request.MaxWaitMillis, request.MinBytes, request.MaxBytes = 1, 500, 1, 4<<20
	p := kmsg.NewFetchRequestTopicPartition()
	p.Partition, p.CurrentLeaderEpoch, p.FetchOffset, p.PartitionMaxBytes = part.key.partition, epoch, next, 1<<20
	request.Topics = []kmsg.FetchRequestTopic{{Topic: part.key.topic, TopicID: topic.TopicID, Partitions: []kmsg.FetchRequestTopicPartition{p}}}
	response, err := s.client.Broker(int(leader)).Request(ctx, request)
	if err != nil {
		return nil, next, err
	}
	fetched := response.(*kmsg.FetchResponse)
	if err := kerr.ErrorForCode(fetched.ErrorCode); err != nil {
		return nil, next, err
	}
	for _, t := range fetched.Topics {
		if t.Topic != part.key.topic && t.TopicID != topic.TopicID {
			continue
		}
		for _, rp := range t.Partitions {
			if rp.Partition != part.key.partition {
				continue
			}
			return processBounded(part, next, &rp)
		}
	}
	return nil, next, errors.New("kafka: bounded fetch omitted requested partition")
}

func processBounded(part boundedPartition, next int64, rp *kmsg.FetchResponseTopicPartition) ([]*kgo.Record, int64, error) {
	fp, advanced := kgo.ProcessFetchPartition(kgo.ProcessFetchPartitionOpts{Topic: part.key.topic, Partition: part.key.partition, Offset: next, IsolationLevel: kgo.ReadCommitted()}, rp, kgo.DefaultDecompressor(), nil)
	if fp.Err != nil {
		return nil, next, fp.Err
	}
	if next < fp.LogStartOffset || fp.LastStableOffset < part.end {
		return nil, next, errors.New("kafka: bounded offset range is no longer retained or stable")
	}
	// An empty successful response at/above our offset proves a compacted
	// tail has no records left. Partial/nonempty undecodable batches do not.
	if advanced == next {
		if len(rp.RecordBatches) != 0 {
			return nil, next, errors.New("kafka: bounded fetch made no progress")
		}
		advanced = part.end
	}
	records := fp.Records[:0]
	for _, record := range fp.Records {
		if record.Offset < part.end {
			records = append(records, record)
		}
	}
	return records, min(advanced, part.end), nil
}

// Extract reads the captured retained window without committing group offsets
// or advancing continuous checkpoints. A new run rereads the retained log.
func (s *Source) Extract(ctx context.Context, out filament.RecordSink, opts filament.ExtractOpts) error {
	if opts.Limit < 0 {
		return errors.New("kafka: limit must be nonnegative")
	}
	selected, err := s.PlanResources(ctx, opts.Resources, opts.Selectors)
	if err != nil {
		return err
	}
	count := 0
	for _, topic := range selected {
		plan := s.bounded[topic]
		writer, err := out.Builder(topic, 0, plan.schema)
		if err != nil {
			return err
		}
		projector := streamkit.NewEventMetadataProjector(writer, s)
		for _, part := range plan.partitions {
			for next := part.start; next < part.end; {
				records, advanced, err := s.fetchBounded(ctx, part, next)
				if err != nil {
					return fmt.Errorf("kafka: bounded read %s partition %d: %w", topic, part.key.partition, err)
				}
				for _, record := range records {
					if err := ctx.Err(); err != nil {
						return err
					}
					if err := appendRecord(writer, projector, plan.columns, part.domain, record); err != nil {
						return err
					}
					count++
					if opts.Limit > 0 && count >= opts.Limit {
						return nil
					}
				}
				next = advanced
			}
		}
	}
	return ctx.Err()
}
