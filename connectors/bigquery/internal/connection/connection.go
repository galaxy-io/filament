package connection

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
)

// Open constructs a BigQuery client using Application Default Credentials.
func Open(ctx context.Context, resolved Resolved) (*bigquery.Client, error) {
	client, err := bigquery.NewClient(ctx, resolved.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	client.Location = resolved.Location
	return client, nil
}

// Test executes a trivial query through a short-lived BigQuery client.
func Test(ctx context.Context, resolved Resolved) error {
	client, err := Open(ctx, resolved)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	query := client.Query("SELECT 1")
	query.Location = resolved.Location
	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run connectivity query: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait for connectivity query: %w", err)
	}
	if err := status.Err(); err != nil {
		return fmt.Errorf("connectivity query: %w", err)
	}
	return nil
}
