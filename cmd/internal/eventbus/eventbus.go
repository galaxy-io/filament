// Package eventbus selects the event-plane transport.
package eventbus

import (
	"errors"
	"fmt"
	"log"
	"os"

	bus "github.com/galaxy-io/filament/eventbus"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
)

// FromEnv selects the transport per EVENTBUS_PROVIDER; nats is the default.
// nats connects to NATS_URL and applies NATS_STREAM and NATS_SUBJECTS when
// set. The NATS provider reads the process-wide NATS_TTL_SECONDS setting.
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
		return natsbus.New(url, events.Codec, opts...)
	default:
		return nil, fmt.Errorf("unknown EVENTBUS_PROVIDER %q (nats)", provider)
	}
}
