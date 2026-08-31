// Package eventbus selects the event-plane transport.
package eventbus

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	bus "github.com/galaxy-io/filament/eventbus"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
)

// FromEnv selects the transport per EVENTBUS_PROVIDER; nats is the default.
// nats connects to NATS_URL and applies NATS_STREAM, NATS_SUBJECTS, and
// NATS_TTL_SECONDS when set.
func FromEnv() (bus.Bus, error) {
	switch provider := os.Getenv("EVENTBUS_PROVIDER"); provider {
	case "", "nats":
		url := os.Getenv("NATS_URL")
		if url == "" {
			return nil, errors.New("NATS_URL is required")
		}
		opts := []natsbus.Option{natsbus.WithLogf(log.Printf)}
		if stream := os.Getenv("NATS_STREAM"); stream != "" {
			opts = append(opts, natsbus.WithStream(stream))
		}
		if subjects := os.Getenv("NATS_SUBJECTS"); subjects != "" {
			opts = append(opts, natsbus.WithSubjects(subjects))
		}
		if raw := os.Getenv("NATS_TTL_SECONDS"); raw != "" {
			seconds, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || seconds < 0 {
				return nil, fmt.Errorf("NATS_TTL_SECONDS must be a non-negative integer, got %q", raw)
			}
			opts = append(opts, natsbus.WithTTL(time.Duration(seconds)*time.Second))
		}
		return natsbus.New(url, events.Codec, opts...)
	default:
		return nil, fmt.Errorf("unknown EVENTBUS_PROVIDER %q (nats)", provider)
	}
}
