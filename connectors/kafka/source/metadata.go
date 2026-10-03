package source

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
)

type partitionKey struct {
	topic     string
	partition int32
}
type partitionState struct {
	domain  filament.DomainKey
	next    int64
	initial bool
}

func domains(m *kmsg.MetadataResponse, identity string) (map[partitionKey]filament.DomainKey, error) {
	if identity == "" || m.ClusterID == nil || *m.ClusterID == "" {
		return nil, errors.New("kafka: stable source and cluster identities required")
	}
	out := map[partitionKey]filament.DomainKey{}
	for _, t := range m.Topics {
		if t.Topic == nil || t.TopicID == [16]byte{} {
			return nil, errors.New("kafka: broker must expose stable topic IDs")
		}
		incarnation, _ := json.Marshal([]string{identity, *m.ClusterID, hex.EncodeToString(t.TopicID[:])})
		for _, p := range t.Partitions {
			domain, _ := json.Marshal([]any{*t.Topic, p.Partition})
			out[partitionKey{*t.Topic, p.Partition}] = filament.DomainKey{Domain: string(domain), Incarnation: string(incarnation)}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("kafka: no topic partitions")
	}
	return out, nil
}

func listOffsets(ctx context.Context, c *kgo.Client, keys map[partitionKey]filament.DomainKey, timestamp int64) (map[partitionKey]int64, error) {
	req := kmsg.NewPtrListOffsetsRequest()
	req.IsolationLevel = 1
	byTopic := map[string][]kmsg.ListOffsetsRequestTopicPartition{}
	for k := range keys {
		p := kmsg.NewListOffsetsRequestTopicPartition()
		p.Partition = k.partition
		p.Timestamp = timestamp
		byTopic[k.topic] = append(byTopic[k.topic], p)
	}
	for t, ps := range byTopic {
		req.Topics = append(req.Topics, kmsg.ListOffsetsRequestTopic{Topic: t, Partitions: ps})
	}
	res, err := req.RequestWith(ctx, c)
	if err != nil {
		return nil, err
	}
	out := map[partitionKey]int64{}
	for _, t := range res.Topics {
		for _, p := range t.Partitions {
			if err := kerr.ErrorForCode(p.ErrorCode); err != nil {
				return nil, err
			}
			if p.Offset < 0 {
				return nil, errors.New("kafka: invalid broker offset")
			}
			out[partitionKey{t.Topic, p.Partition}] = p.Offset
		}
	}
	for k := range keys {
		if _, ok := out[k]; !ok {
			return nil, errors.New("kafka: missing broker offset")
		}
	}
	return out, nil
}

func (s *session) validateMetadata(ctx context.Context) error {
	m, err := client.Metadata(ctx, s.metadata, s.topics)
	if err != nil {
		return err
	}
	ds, err := domains(m, s.identity)
	if err != nil {
		return err
	}
	if len(ds) != len(s.partitions) {
		return errors.New("kafka: partition membership changed; restart with admitted membership")
	}
	for k, d := range ds {
		p, ok := s.partitions[k]
		if !ok || p.domain != d {
			return filament.ErrPositionIncomparable
		}
	}
	starts, err := listOffsets(ctx, s.metadata, ds, -2)
	if err != nil {
		return err
	}
	ends, err := listOffsets(ctx, s.metadata, ds, -1)
	if err != nil {
		return err
	}
	for k, p := range s.partitions {
		if p.next < starts[k] || p.next > ends[k] {
			return fmt.Errorf("kafka: certified position outside retained range for %s partition %d", k.topic, k.partition)
		}
	}
	return nil
}
