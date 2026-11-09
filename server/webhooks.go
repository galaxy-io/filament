package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

var errWebhooksUnavailable = errors.New("webhook endpoints are not available yet")

func (*Server) GetWebhookEndpoint(context.Context, *connect.Request[ingestionv1.GetWebhookEndpointRequest]) (*connect.Response[ingestionv1.GetWebhookEndpointResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errWebhooksUnavailable)
}

func (*Server) PutWebhookEndpoint(context.Context, *connect.Request[ingestionv1.PutWebhookEndpointRequest]) (*connect.Response[ingestionv1.PutWebhookEndpointResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errWebhooksUnavailable)
}
