package sink

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/nats/internal/connection"
)

const (
	defaultMaxInFlight      = 1024
	defaultMaxInFlightBytes = 16 << 20
	maxPublishConcurrency   = 65536
)

// Subject routing modes. Prefix is the default: each destination resource
// publishes to its own subject beneath the prefix.
const (
	routingField  = "subject_mode"
	routingPrefix = "prefix"
	routingFixed  = "fixed"
)

type config struct {
	stream, subject, prefix string
	timeout                 time.Duration
	maxInFlight             int
	maxInFlightBytes        int64
}

// Validate checks connection settings independently of pipeline settings.
func (*Sink) Validate(cfg filament.Config) error { return connection.Validate(cfg) }

func resolve(cfg filament.Config) (config, error) {
	c := config{stream: cfg.String("stream"), timeout: 5 * time.Second, maxInFlight: defaultMaxInFlight, maxInFlightBytes: defaultMaxInFlightBytes}
	if c.stream == "" {
		return c, fmt.Errorf("nats sink: stream is required")
	}
	// The selector decides which subject field applies; the other is ignored so
	// a stale value left behind by a form switch cannot cause a conflict.
	mode := cfg.String(routingField)
	if mode == "" {
		mode = routingPrefix
	}
	var target string
	switch mode {
	case routingPrefix:
		c.prefix = cfg.String("subject_prefix")
		if c.prefix == "" {
			return c, fmt.Errorf("nats sink: subject_prefix is required for prefix routing")
		}
		target = c.prefix
	case routingFixed:
		c.subject = cfg.String("subject")
		if c.subject == "" {
			return c, fmt.Errorf("nats sink: subject is required for fixed routing")
		}
		target = c.subject
	default:
		return c, fmt.Errorf("nats sink: %s must be %q or %q", routingField, routingPrefix, routingFixed)
	}
	if err := validSubject(target); err != nil {
		return c, err
	}
	if cfg.Has("publish_timeout") {
		var err error
		c.timeout, err = time.ParseDuration(cfg.String("publish_timeout"))
		if err != nil || c.timeout <= 0 {
			return c, fmt.Errorf("nats sink: publish_timeout must be a positive duration")
		}
	}
	limit, err := positiveInt(cfg, "max_in_flight", defaultMaxInFlight)
	if err != nil {
		return c, err
	}
	if limit > maxPublishConcurrency {
		return c, fmt.Errorf("nats sink: max_in_flight must not exceed %d", maxPublishConcurrency)
	}
	c.maxInFlight = int(limit)
	c.maxInFlightBytes, err = positiveInt(cfg, "max_in_flight_bytes", defaultMaxInFlightBytes)
	if err != nil {
		return c, err
	}
	return c, nil
}

func validSubject(subject string) error {
	if subject == "" || strings.ContainsAny(subject, "*>") || strings.ContainsFunc(subject, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("nats sink: invalid publish subject %q", subject)
	}
	for _, token := range strings.Split(subject, ".") {
		if token == "" {
			return fmt.Errorf("nats sink: empty subject token in %q", subject)
		}
	}
	return nil
}

func (c config) route(resource string) (string, error) {
	subject := c.subject
	if subject == "" {
		subject = c.prefix + "." + resource
	}
	return subject, validSubject(subject)
}

// Decode integer config without silently truncating fractional JSON numbers.
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
	return 0, fmt.Errorf("nats sink: %s must be a positive integer", key)
}
