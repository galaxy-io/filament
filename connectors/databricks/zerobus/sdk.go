package zerobus

import (
	"context"
	"errors"
	"fmt"

	zb "github.com/databricks/zerobus-sdk/go"

	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
)

// sdkClient adapts the official Zerobus Go SDK (a CGO wrapper over the Rust
// core) to the ingestClient interface. This is the only file that imports the
// SDK, keeping the CGO dependency isolated from the rest of the connector.
type sdkClient struct {
	sdk      *zb.ZerobusSdk
	resolved connection.Resolved
	comp     zb.IPCCompressionType
}

// newSDKClient is the default clientFactory. It constructs the SDK handle;
// per-table streams are opened lazily via OpenStream.
func newSDKClient(r connection.Resolved, compression string) (ingestClient, error) {
	sdk, err := zb.NewZerobusSdk(r.ZerobusEndpoint, r.WorkspaceURL)
	if err != nil {
		return nil, fmt.Errorf("create zerobus sdk: %w", err)
	}
	return &sdkClient{sdk: sdk, resolved: r, comp: compressionFor(compression)}, nil
}

// validateCompression rejects an ipc_compression value that is not a known
// codec. An empty value is accepted and treated as "none". Without this,
// compressionFor would silently map a misspelled value to no compression.
func validateCompression(s string) error {
	switch s {
	case "", connection.CompressionNone, connection.CompressionLZ4, connection.CompressionZstd:
		return nil
	default:
		return fmt.Errorf("unsupported ipc_compression %q (want none, lz4, or zstd)", s)
	}
}

// compressionFor maps the ipc_compression config value to the SDK codec.
// The codec is applied by the SDK on the Arrow Flight wire; callers always hand
// IngestBatch uncompressed IPC bytes.
func compressionFor(s string) zb.IPCCompressionType {
	switch s {
	case connection.CompressionLZ4:
		return zb.IPCCompressionLZ4Frame
	case connection.CompressionZstd:
		return zb.IPCCompressionZstd
	default:
		return zb.IPCCompressionNone
	}
}

// OpenStream creates an Arrow Flight stream for the fully qualified table,
// applying the configured IPC compression codec.
func (c *sdkClient) OpenStream(ctx context.Context, table string, schemaIPC []byte) (ingestStream, error) {
	opts := zb.DefaultArrowStreamConfigurationOptions()
	opts.IPCCompression = c.comp
	stream, err := c.sdk.CreateArrowStream(table, schemaIPC, c.resolved.ClientID, c.resolved.ClientSecret, opts)
	if err != nil {
		return nil, err
	}
	return &sdkStream{stream: stream}, nil
}

// Close frees the underlying SDK handle. It is safe to call more than once.
func (c *sdkClient) Close() error {
	if c.sdk != nil {
		c.sdk.Free()
		c.sdk = nil
	}
	return nil
}

// sdkStream adapts a single Zerobus Arrow Flight stream to ingestStream.
type sdkStream struct{ stream *zb.ZerobusArrowStream }

// Ingest sends one Arrow IPC batch and returns the assigned offset.
func (s *sdkStream) Ingest(ipc []byte) (int64, error) { return s.stream.IngestBatch(ipc) }

// Flush blocks until every ingested batch is durably stored.
func (s *sdkStream) Flush() error { return s.stream.Flush() }

// Close releases the stream handle.
func (s *sdkStream) Close() error { return s.stream.Close() }

// retryable reports whether err is a retryable Zerobus error, for telemetry.
func retryable(err error) bool {
	var ze *zb.ZerobusError
	if errors.As(err, &ze) {
		return ze.Retryable()
	}
	return false
}
