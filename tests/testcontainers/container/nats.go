package container

import (
	"context"
	"fmt"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NATSOptions configures the NATS container.
type NATSOptions struct {
	Name    string // required: logical name for gx state
	Image   string // default: NATS_IMAGE from docker/.env
	Network string // optional: Docker network name to join
}

// StartNATS starts a NATS container and registers it in the gx state file.
func StartNATS(ctx context.Context, opts NATSOptions) (Entry, error) {
	disableRyuk()
	if opts.Image == "" {
		var err error
		opts.Image, err = Image("NATS_IMAGE")
		if err != nil {
			return Entry{}, err
		}
	}
	if err := ensureNetwork(ctx, opts.Network); err != nil {
		return Entry{}, err
	}

	req := tc.ContainerRequest{
		Name:         "gx-" + opts.Name,
		Image:        opts.Image,
		Cmd:          []string{"-js"},
		ExposedPorts: []string{"4222/tcp"},
		WaitingFor:   wait.ForLog("Server is ready"),
	}
	if opts.Network != "" {
		req.Networks = []string{opts.Network}
	}

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return Entry{}, fmt.Errorf("start nats: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, err
	}
	port, err := ctr.MappedPort(ctx, "4222")
	if err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, err
	}

	id := ctr.GetContainerID()
	if id == "" {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, fmt.Errorf("container id empty")
	}

	e := Entry{
		Name:      opts.Name,
		Type:      "nats",
		ID:        id,
		DSN:       fmt.Sprintf("nats://%s:%s", host, port.Port()),
		Image:     opts.Image,
		Network:   opts.Network,
		CreatedAt: time.Now(),
	}
	if err := addEntry(e); err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, err
	}
	return e, nil
}
