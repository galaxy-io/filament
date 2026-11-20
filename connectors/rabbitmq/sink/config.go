package sink

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/rabbitmq/internal/connection"
)

const (
	defaultMaxInFlight      = 1024
	defaultMaxInFlightBytes = 16 << 20
	maxPublishConcurrency   = 65536
	maxRecordBytes          = 100 << 20
	maxStreamNameBytes      = 255
)

// resourcePlaceholder expands to the destination resource name. The Go template
// spelling {{resource}} is accepted as well.
const resourcePlaceholder = "{resource}"

var resourcePlaceholders = []string{"{{resource}}", resourcePlaceholder}

type template string

func (t template) expand(resource string) string {
	out := string(t)
	for _, placeholder := range resourcePlaceholders {
		out = strings.ReplaceAll(out, placeholder, resource)
	}
	return out
}

func (t template) perResource() bool {
	for _, placeholder := range resourcePlaceholders {
		if strings.Contains(string(t), placeholder) {
			return true
		}
	}
	return false
}

type config struct {
	stream                           template
	timeout                          time.Duration
	maxInFlight                      int
	maxInFlightBytes, maxRecordBytes int64
}

// Validate checks the connection URI independently of pipeline settings.
func (*Sink) Validate(cfg filament.Config) error { return connection.Validate(cfg) }

func resolve(cfg filament.Config) (config, error) {
	c := config{stream: template(cfg.String("stream")), timeout: 30 * time.Second, maxInFlight: defaultMaxInFlight, maxInFlightBytes: defaultMaxInFlightBytes, maxRecordBytes: 1 << 20}
	if c.stream == "" {
		return c, fmt.Errorf("rabbitmq sink: stream is required")
	}
	if err := validStreamName(c.stream.expand("resource")); err != nil {
		return c, err
	}
	if cfg.Has("publish_timeout") {
		d, err := time.ParseDuration(cfg.String("publish_timeout"))
		if err != nil || d < time.Second {
			return c, fmt.Errorf("rabbitmq sink: publish_timeout must be at least 1s")
		}
		c.timeout = d
	}
	limit, err := positiveInt(cfg, "max_in_flight", defaultMaxInFlight)
	if err != nil {
		return c, err
	}
	if limit > maxPublishConcurrency {
		return c, fmt.Errorf("rabbitmq sink: max_in_flight must not exceed %d", maxPublishConcurrency)
	}
	c.maxInFlight = int(limit)
	c.maxInFlightBytes, err = positiveInt(cfg, "max_in_flight_bytes", defaultMaxInFlightBytes)
	if err != nil {
		return c, err
	}
	c.maxRecordBytes, err = positiveInt(cfg, "max_record_bytes", 1<<20)
	if err != nil {
		return c, err
	}
	if c.maxRecordBytes < 512 || c.maxRecordBytes > maxRecordBytes {
		return c, fmt.Errorf("rabbitmq sink: max_record_bytes must be between 512 bytes and 100 MiB")
	}
	if c.maxRecordBytes > c.maxInFlightBytes || c.maxInFlightBytes > 1<<30 {
		return c, fmt.Errorf("rabbitmq sink: record limit must fit outstanding byte limit; limits must not exceed 1 GiB")
	}
	return c, nil
}

func validStreamName(name string) error {
	if name == "" || len(name) > maxStreamNameBytes || strings.ContainsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("rabbitmq sink: invalid stream name %q", name)
	}
	return nil
}

func positiveInt(cfg filament.Config, key string, fallback int64) (int64, error) {
	if !cfg.Has(key) {
		return fallback, nil
	}
	raw, err := json.Marshal(cfg.Raw()[key])
	if err == nil {
		value, parseErr := strconv.ParseInt(string(raw), 10, 64)
		if parseErr == nil && value > 0 {
			return value, nil
		}
	}
	return 0, fmt.Errorf("rabbitmq sink: %s must be a positive integer", key)
}
