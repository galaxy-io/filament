package container

import (
	"context"
	"fmt"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RedisOptions configures the Redis container.
type RedisOptions struct {
	Name    string // required: logical name for gx state
	Image   string // default: REDIS_IMAGE from docker/.env
	Network string // optional: Docker network name to join
}

// StartRedis starts a Redis container and registers it in the gx state file.
func StartRedis(ctx context.Context, opts RedisOptions) (Entry, error) {
	disableRyuk()
	if opts.Image == "" {
		var err error
		opts.Image, err = Image("REDIS_IMAGE")
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
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	if opts.Network != "" {
		req.Networks = []string{opts.Network}
	}

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return Entry{}, fmt.Errorf("start redis: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, err
	}
	port, err := ctr.MappedPort(ctx, "6379")
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
		Type:      "redis",
		ID:        id,
		DSN:       fmt.Sprintf("redis://%s:%s", host, port.Port()),
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
