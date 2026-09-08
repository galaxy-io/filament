//go:build integration || e2e

package testcontainers

import (
	"context"
	"testing"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
)

// MinIO holds an ephemeral MinIO container and a connected S3 client.
type MinIO struct {
	Container *tcminio.MinioContainer
	Endpoint  string // host:port of the S3 API
	AccessKey string
	SecretKey string
	Client    *miniogo.Client
}

// MinIOContainer starts a MinIO container (image from MINIO_IMAGE in docker/.env),
// connects an S3 client, and registers cleanup.
func MinIOContainer(t testing.TB) *MinIO {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcminio.Run(ctx, Image(t, "MINIO_IMAGE"),
		tcminio.WithUsername("minioadmin"),
		tcminio.WithPassword("minioadmin"),
	)
	if err != nil {
		t.Fatalf("start minio container: %v", err)
	}
	cleanupContainer(t, "minio", ctr)

	endpoint, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio endpoint: %v", err)
	}

	client, err := miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(ctr.Username, ctr.Password, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}

	return &MinIO{
		Container: ctr,
		Endpoint:  endpoint,
		AccessKey: ctr.Username,
		SecretKey: ctr.Password,
		Client:    client,
	}
}
