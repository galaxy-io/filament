// Package remote implements CLI target operations against a deployed
// Filament service over its Connect API.
package remote

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var _ cliapp.Target = (*Target)(nil)

// TokenSource supplies a valid bearer token for outgoing requests.
type TokenSource interface {
	Token(context.Context) (string, error)
}

// Options configures a remote target.
type Options struct {
	// Endpoint is the deployment's base URL.
	Endpoint string
	// Tokens authenticates requests; nil sends them unauthenticated.
	Tokens TokenSource
	// HTTPClient issues requests; nil means the default client.
	HTTPClient *http.Client
}

// Target implements CLI operations over a deployment's IngestionService.
type Target struct {
	client   ingestionv1connect.IngestionServiceClient
	endpoint string
}

// NewTarget constructs a remote target.
func NewTarget(opts Options) *Target {
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	var options []connect.ClientOption
	if opts.Tokens != nil {
		options = append(options, connect.WithInterceptors(requestInterceptor{tokens: opts.Tokens}))
	}
	return &Target{
		client:   ingestionv1connect.NewIngestionServiceClient(client, opts.Endpoint, options...),
		endpoint: opts.Endpoint,
	}
}

// ValidateConfiguration is satisfied server-side: every remote mutation and
// run is validated by the deployment when it happens.
func (t *Target) ValidateConfiguration(context.Context, model.Document) error { return nil }

// requestInterceptor authenticates every RPC in one place so an adapter
// cannot accidentally omit it. Tenancy is the server's side of the token:
// the deployment resolves it from the caller, never from the request.
type requestInterceptor struct {
	tokens TokenSource
}

func (i requestInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if i.tokens != nil {
			token, err := i.tokens.Token(ctx)
			if err != nil {
				return nil, err
			}
			req.Header().Set("Authorization", "Bearer "+token)
		}
		return next(ctx, req)
	}
}

func (i requestInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		wrapped := &requestClientConn{StreamingClientConn: conn}
		if i.tokens != nil {
			token, err := i.tokens.Token(ctx)
			if err != nil {
				wrapped.err = err
				return wrapped
			}
			conn.RequestHeader().Set("Authorization", "Bearer "+token)
		}
		return wrapped
	}
}

func (i requestInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type requestClientConn struct {
	connect.StreamingClientConn
	err error
}

func (c *requestClientConn) Send(message any) error {
	if c.err != nil {
		return c.err
	}
	return c.StreamingClientConn.Send(message)
}

func (c *requestClientConn) Receive(message any) error {
	if c.err != nil {
		return c.err
	}
	return c.StreamingClientConn.Receive(message)
}

// rpcError translates transport and RPC failures into CLI-ready messages.
func (t *Target) rpcError(err error) error {
	if err == nil {
		return nil
	}
	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeUnauthenticated:
		return fmt.Errorf("%s rejected the request as unauthenticated; run filament auth login", t.endpoint)
	case connect.CodeUnavailable:
		return fmt.Errorf("cannot reach %s: %s", t.endpoint, connectErr.Message())
	default:
		return errors.New(connectErr.Message())
	}
}

func connectorKind(kind string) (ingestionv1.ConnectorKind, error) {
	switch kind {
	case "source":
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, nil
	case "sink":
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, nil
	default:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED, fmt.Errorf("unknown connection kind %q", kind)
	}
}

func configStruct(config map[string]any) (*structpb.Struct, error) {
	if len(config) == 0 {
		return nil, nil
	}
	value, err := structpb.NewStruct(config)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return value, nil
}

func configMap(value *structpb.Struct) map[string]any {
	if value == nil || len(value.GetFields()) == 0 {
		return nil
	}
	return value.AsMap()
}

func revisionOf(version int64) string {
	return strconv.FormatInt(version, 10)
}
