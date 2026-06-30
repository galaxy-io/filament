package container

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TrinoOptions configures the Trino container.
type TrinoOptions struct {
	Name    string // required: logical name for gx state
	Catalog string // default: "memory"
	Image   string // default: TRINO_IMAGE from docker/.env
	Network string // optional: Docker network name to join
}

// StartTrino starts a Trino container and registers it in the gx state file. It
// uses the built-in memory catalog, so no external metastore is needed.
func StartTrino(ctx context.Context, opts TrinoOptions) (Entry, error) {
	disableRyuk()
	if opts.Catalog == "" {
		opts.Catalog = "memory"
	}
	if opts.Image == "" {
		img, err := Image("TRINO_IMAGE")
		if err != nil {
			return Entry{}, err
		}
		opts.Image = img
	}
	if err := ensureNetwork(ctx, opts.Network); err != nil {
		return Entry{}, err
	}

	req := tc.ContainerRequest{
		Name:         "gx-" + opts.Name,
		Image:        opts.Image,
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor: wait.ForHTTP("/v1/info").WithPort("8080/tcp").
			WithResponseMatcher(func(body io.Reader) bool {
				b, _ := io.ReadAll(body)
				return strings.Contains(string(b), `"starting":false`)
			}).WithStartupTimeout(3 * time.Minute),
	}
	if opts.Network != "" {
		req.Networks = []string{opts.Network}
	}

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return Entry{}, fmt.Errorf("start trino: %w", err)
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		_ = tc.TerminateContainer(ctr)
		return Entry{}, err
	}
	port, err := ctr.MappedPort(ctx, "8080")
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
		Type:      "trino",
		ID:        id,
		DSN:       fmt.Sprintf("http://test@%s:%s?catalog=%s", host, port.Port(), opts.Catalog),
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
