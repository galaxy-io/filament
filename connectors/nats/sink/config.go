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
	"github.com/galaxy-io/filament/connectors/nats/internal/subject"
)

const (
	defaultMaxInFlight      = 1024
	defaultMaxInFlightBytes = 16 << 20
	maxPublishConcurrency   = 65536
)

// resourcePlaceholder expands to the destination resource name inside the
// stream and subject templates.
const resourcePlaceholder = "{resource}"

type template string

func (t template) expand(resource string) string {
	return strings.ReplaceAll(string(t), resourcePlaceholder, resource)
}

func (t template) perResource() bool { return strings.Contains(string(t), resourcePlaceholder) }

type config struct {
	stream, subject  template
	createStream     bool
	timeout          time.Duration
	maxInFlight      int
	maxInFlightBytes int64
}

// Validate checks connection settings independently of pipeline settings.
func (*Sink) Validate(cfg filament.Config) error { return connection.Validate(cfg) }

func resolve(cfg filament.Config) (config, error) {
	c := config{stream: template(cfg.String("stream")), subject: template(cfg.String("subject")), timeout: 5 * time.Second, maxInFlight: defaultMaxInFlight, maxInFlightBytes: defaultMaxInFlightBytes}
	if c.stream == "" {
		return c, fmt.Errorf("nats sink: stream is required")
	}
	if c.subject == "" {
		return c, fmt.Errorf("nats sink: subject is required")
	}
	// Templates must be valid once a well-formed resource name is substituted.
	if err := validStreamName(c.stream.expand("resource")); err != nil {
		return c, err
	}
	if err := validSubject(c.subject.expand("resource")); err != nil {
		return c, err
	}
	c.createStream = !cfg.Has("create_stream") || cfg.Bool("create_stream")
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

func validStreamName(name string) error {
	if name == "" || strings.ContainsAny(name, ".*>/\\") || strings.ContainsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return fmt.Errorf("nats sink: invalid stream name %q: use letters, digits, - and _; dots, spaces, and wildcards are not allowed", name)
	}
	return nil
}

// captureFilter is the subject filter a created stream needs for a resource.
// A shared stream with a per-resource subject captures every expansion: > when
// the placeholder ends the subject, * otherwise. Any other case is exact.
func (c config) captureFilter(resource string) string {
	if c.subject.perResource() && !c.stream.perResource() {
		if strings.HasSuffix(string(c.subject), resourcePlaceholder) {
			return c.subject.expand(">")
		}
		return c.subject.expand("*")
	}
	return c.subject.expand(resource)
}

// destination is one resolved stream: its subject filters and size limit.
type destination struct {
	name       string
	filters    []string
	maxPayload int64
}

// route resolves the concrete subject for a resource and verifies the
// destination captures it, so a misrouted batch fails before any publish.
func (c config) route(resource string, d *destination) (string, error) {
	target := c.subject.expand(resource)
	if err := validSubject(target); err != nil {
		return "", fmt.Errorf("%w for resource %q", err, resource)
	}
	if !subject.Covered(d.filters, target) {
		return "", fmt.Errorf("nats sink: stream %q subjects %v do not capture %q for resource %q", d.name, d.filters, target, resource)
	}
	return target, nil
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
