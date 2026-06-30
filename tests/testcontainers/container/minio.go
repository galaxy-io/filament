package container

import (
	"context"
	"fmt"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

// MinIOOptions configures the MinIO container.
type MinIOOptions struct {
	Name     string // required: logical name for gx state
	Username string // default: "minioadmin"
	Password string // default: "minioadmin"
	Image    string // default: MINIO_IMAGE from docker/.env
	Network  string // optional: Docker network name to join
}

// StartMinIO starts a MinIO container and registers it in the gx state file. The
// DSN embeds the credentials so it is directly usable as an S3 endpoint.
func StartMinIO(ctx context.Context, opts MinIOOptions) (Entry, error) {
	disableRyuk()
	if opts.Username == "" {
		opts.Username = "minioadmin"
	}
	if opts.Password == "" {
		opts.Password = "minioadmin"
	}
	if opts.Image == "" {
		img, err := Image("MINIO_IMAGE")
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
		tcminio.WithUsername(opts.Username),
		tcminio.WithPassword(opts.Password),
		tc.CustomizeRequest(tc.GenericContainerRequest{ContainerRequest: nameReq}),
	}

	ctr, err := tcminio.Run(ctx, opts.Image, runOpts...)
	if err != nil {
		return Entry{}, fmt.Errorf("start minio: %w", err)
	}

	endpoint, err := ctr.ConnectionString(ctx)
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
		Type:      "minio",
		ID:        id,
		DSN:       fmt.Sprintf("http://%s:%s@%s", opts.Username, opts.Password, endpoint),
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
