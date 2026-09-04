// Package remote implements CLI target operations against a deployed
// Filament service over its Connect API. The CLI's local mode uses it too,
// pointed at a server embedded in the same process.
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

var (
	_ cliapp.Target              = (*Target)(nil)
	_ cliapp.PipelineModesTarget = (*Target)(nil)
)

// TokenSource supplies a bearer token for outgoing requests. A source that
// also implements Invalidate gets one retry after the server rejects a token.
type TokenSource interface {
	Token(context.Context) (string, error)
}

// Options configures a remote target.
type Options struct {
	// Endpoint is the deployment's base URL.
	Endpoint string
	// Tokens authenticates requests; nil sends them unauthenticated, which
	// only an embedded loopback deployment accepts.
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
		options = append(options, connect.WithInterceptors(bearerInterceptor{tokens: opts.Tokens}))
	}
	return &Target{
		client:   ingestionv1connect.NewIngestionServiceClient(client, opts.Endpoint, options...),
		endpoint: opts.Endpoint,
	}
}

// ValidateConfiguration is satisfied server-side: the deployment validates
// every mutation and run when it happens.
func (t *Target) ValidateConfiguration(context.Context, model.Document) error { return nil }

// bearerInterceptor authenticates every RPC in one place. Tenancy is the
// server's side of the token; requests never carry it.
type bearerInterceptor struct {
	tokens TokenSource
}

type invalidator interface {
	Invalidate() error
}

func (i bearerInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.authorize(ctx, req.Header()); err != nil {
			return nil, err
		}
		resp, err := next(ctx, req)
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			return resp, err
		}
		// A cached token the server no longer accepts is dropped and minted
		// once more before the failure reaches the user.
		source, ok := i.tokens.(invalidator)
		if !ok {
			return resp, err
		}
		if err := source.Invalidate(); err != nil {
			return nil, err
		}
		if err := i.authorize(ctx, req.Header()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i bearerInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		wrapped := &streamConn{StreamingClientConn: conn}
		wrapped.err = i.authorize(ctx, conn.RequestHeader())
		return wrapped
	}
}

func (i bearerInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func (i bearerInterceptor) authorize(ctx context.Context, header http.Header) error {
	token, err := i.tokens.Token(ctx)
	if err != nil {
		return err
	}
	header.Set("Authorization", "Bearer "+token)
	return nil
}

// streamConn surfaces a token failure from the first Send or Receive, since
// the interceptor cannot return an error while opening the stream.
type streamConn struct {
	connect.StreamingClientConn
	err error
}

func (c *streamConn) Send(message any) error {
	if c.err != nil {
		return c.err
	}
	return c.StreamingClientConn.Send(message)
}

func (c *streamConn) Receive(message any) error {
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
	var connectErr *connect.Error
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

func kindString(kind ingestionv1.ConnectorKind) string {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		return "source"
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		return "sink"
	default:
		return ""
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
