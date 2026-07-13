package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// PutSecret rejects the request; secrets are not configured in this binary.
func (a *Server) PutSecret(_ context.Context, _ *connect.Request[ingestionv1.PutSecretRequest]) (*connect.Response[ingestionv1.PutSecretResponse], error) {
	return nil, fmt.Errorf("secrets are not configured in this ingestion binary")
}

// DeleteSecret rejects the request; secrets are not configured in this binary.
func (a *Server) DeleteSecret(_ context.Context, _ *connect.Request[ingestionv1.DeleteSecretRequest]) (*connect.Response[ingestionv1.DeleteSecretResponse], error) {
	return nil, fmt.Errorf("secrets are not configured in this ingestion binary")
}
