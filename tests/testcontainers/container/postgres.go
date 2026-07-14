package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// PostgresOptions configures the Postgres container.
type PostgresOptions struct {
	Name     string // required: logical name for gx state
	Database string // default: "test"
	Username string // default: "test"
	Password string // default: "test"
	Image    string // default: POSTGRES_IMAGE from docker/.env
	Network  string // optional: Docker network name to join
}

var ryukOnce sync.Once

// disableRyuk prevents testcontainers from registering containers with the
// Ryuk reaper, which would terminate them when the CLI process exits. CLI
// containers are long-lived — only gx container rm should stop them.
func disableRyuk() {
	ryukOnce.Do(func() { _ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true") })
}

// ensureNetwork creates the named Docker network if it does not already exist.
func ensureNetwork(ctx context.Context, network string) error {
	if network == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", "network", "inspect", network)
	if err := cmd.Run(); err == nil {
		return nil // already exists
	}
	var stderr strings.Builder
	create := exec.CommandContext(ctx, "docker", "network", "create", network)
	create.Stderr = &stderr
	if err := create.Run(); err != nil {
		return fmt.Errorf("docker network create %s: %w\n%s", network, err, stderr.String())
	}
	return nil
}

// StartPostgres starts a Postgres container and registers it in the gx state
// file. The container runs until explicitly stopped with StopContainer.
func StartPostgres(ctx context.Context, opts PostgresOptions) (Entry, error) {
	disableRyuk()
	if opts.Database == "" {
		opts.Database = "test"
	}
	if opts.Username == "" {
		opts.Username = "test"
	}
	if opts.Password == "" {
		opts.Password = "test"
	}
	if opts.Image == "" {
		img, err := Image("POSTGRES_IMAGE")
		if err != nil {
			return Entry{}, err
		}
		opts.Image = img
	}
	if err := ensureNetwork(ctx, opts.Network); err != nil {
		return Entry{}, err
	}

	nameReq := tc.ContainerRequest{Name: "gx-" + opts.Name}
	if opts.Network != "" {
		nameReq.Networks = []string{opts.Network}
	}
	runOpts := []tc.ContainerCustomizer{
		postgres.WithDatabase(opts.Database),
		postgres.WithUsername(opts.Username),
		postgres.WithPassword(opts.Password),
		postgres.BasicWaitStrategies(),
		tc.CustomizeRequest(tc.GenericContainerRequest{ContainerRequest: nameReq}),
	}

	ctr, err := postgres.Run(ctx, opts.Image, runOpts...)
	if err != nil {
		return Entry{}, fmt.Errorf("start postgres: %w", err)
	}

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, fmt.Errorf("connection string: %w", err)
	}

	id := ctr.GetContainerID()
	if id == "" {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, fmt.Errorf("container id empty")
	}

	e := Entry{
		Name:      opts.Name,
		Type:      "postgres",
		ID:        id,
		DSN:       dsn,
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

// StopContainer force-removes the Docker container for a given state entry and
// removes it from the state file. If Docker reports the container is already
// gone, the state entry is still cleaned up.
func StopContainer(ctx context.Context, name string) error {
	e, err := GetEntry(name)
	if err != nil {
		return err
	}

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", e.ID)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// "No such container" means Docker already cleaned it up — still
		// remove from state so the name is free to reuse.
		if !strings.Contains(stderr.String(), "No such container") {
			return fmt.Errorf("docker rm -f %s: %w\n%s", e.ID, err, stderr.String())
		}
	}

	return removeEntry(name)
}
