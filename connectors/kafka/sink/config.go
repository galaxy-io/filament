package sink

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
)

const (
	defaultMaxInFlightBytes = 16 << 20
	// Match franz-go's default BrokerMaxWriteBytes.
	maxRecordBytes = 100 << 20
)

type config struct {
	topic, prefix, format, keyField, partitionField, deleteMode string
	timeout                                                     time.Duration
	maxInFlight                                                 int
	maxInFlightBytes, maxRecordBytes                            int64
}

// Validate checks configuration without contacting the broker.
func (*Sink) Validate(cfg filament.Config) error { return client.Validate(cfg) }

func resolve(cfg filament.Config) (config, error) {
	c := config{topic: cfg.String("topic"), prefix: cfg.String("topic_prefix"), format: cfg.String("format"), keyField: cfg.String("key_field"), partitionField: cfg.String("partition_field"), deleteMode: cfg.String("delete_mode"), timeout: 30 * time.Second}
	if (c.topic == "") == (c.prefix == "") {
		return c, fmt.Errorf("kafka sink: configure exactly one of topic and topic_prefix")
	}
	target := c.topic
	if target == "" {
		target = c.prefix
	}
	if err := client.ValidateTopic(target); err != nil {
		return c, err
	}
	if c.format == "" {
		c.format = "json"
	}
	if c.format != "json" && c.format != "message" {
		return c, fmt.Errorf("kafka sink: format must be json or message")
	}
	if c.format == "message" && c.keyField != "" {
		return c, fmt.Errorf("kafka sink: message format uses the envelope key")
	}
	if c.deleteMode == "" {
		c.deleteMode = "record"
	}
	if c.deleteMode != "record" && c.deleteMode != "tombstone" {
		return c, fmt.Errorf("kafka sink: delete_mode must be record or tombstone")
	}
	if cfg.Has("partitioning") && cfg.String("partitioning") != "key" {
		return c, fmt.Errorf("kafka sink: only key partitioning is supported")
	}
	if cfg.Has("publish_timeout") {
		d, err := time.ParseDuration(cfg.String("publish_timeout"))
		if err != nil || d < time.Second {
			return c, fmt.Errorf("kafka sink: publish_timeout must be at least 1s")
		}
		c.timeout = d
	}
	n, err := positiveInt(cfg, "max_in_flight", 1024)
	if err != nil {
		return c, err
	}
	if n > 65536 {
		return c, fmt.Errorf("kafka sink: max_in_flight exceeds 65536")
	}
	c.maxInFlight = int(n)
	c.maxInFlightBytes, err = positiveInt(cfg, "max_in_flight_bytes", defaultMaxInFlightBytes)
	if err != nil {
		return c, err
	}
	c.maxRecordBytes, err = positiveInt(cfg, "max_record_bytes", 1<<20)
	if err != nil {
		return c, err
	}
	if c.maxRecordBytes < 512 || c.maxRecordBytes > maxRecordBytes {
		return c, fmt.Errorf("kafka sink: max_record_bytes must be between 512 bytes and 100 MiB")
	}
	if c.maxRecordBytes > c.maxInFlightBytes || c.maxInFlightBytes > 1<<30 {
		return c, fmt.Errorf("kafka sink: record limit must fit outstanding byte limit; limits must not exceed 1 GiB")
	}
	return c, nil
}

func positiveInt(cfg filament.Config, key string, fallback int64) (int64, error) {
	if !cfg.Has(key) {
		return fallback, nil
	}
	raw, err := json.Marshal(cfg.Raw()[key])
	if err == nil {
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err == nil && n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("kafka sink: %s must be a positive integer", key)
}

func (c config) route(resource string) (string, error) {
	t := c.topic
	if t == "" {
		t = c.prefix + "." + resource
	}
	return t, client.ValidateTopic(t)
}

// producerOptions keeps client construction consistent with the resolved limits.
func (c config) producerOptions() []kgo.Opt {
	// #nosec G115 -- resolve bounds maxRecordBytes to 1 GiB before this conversion.
	opts := []kgo.Opt{kgo.RequiredAcks(kgo.AllISRAcks()), kgo.MaxBufferedRecords(c.maxInFlight), kgo.MaxBufferedBytes(int(c.maxInFlightBytes)), kgo.RecordDeliveryTimeout(c.timeout), kgo.ProducerBatchMaxBytes(int32(c.maxRecordBytes)), kgo.ProducerLinger(0)}
	if c.partitionField != "" {
		opts = append(opts, kgo.RecordPartitioner(kgo.ManualPartitioner()))
	} else {
		opts = append(opts, kgo.RecordPartitioner(kgo.StickyKeyPartitioner(nil)))
	}
	return opts
}
