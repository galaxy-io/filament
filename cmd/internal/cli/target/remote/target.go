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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	// Tenant scopes requests when the context explicitly selects one.
	Tenant string
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
	if opts.Tokens != nil || opts.Tenant != "" {
		options = append(options, connect.WithInterceptors(requestInterceptor{tokens: opts.Tokens, tenant: opts.Tenant}))
	}
	return &Target{
		client:   ingestionv1connect.NewIngestionServiceClient(client, opts.Endpoint, options...),
		endpoint: opts.Endpoint,
	}
}

// ValidateConfiguration is satisfied server-side: every remote mutation and
// run is validated by the deployment when it happens.
func (t *Target) ValidateConfiguration(context.Context, model.Document) error { return nil }

// requestInterceptor applies deployment-wide metadata in one place so an RPC
// adapter cannot accidentally omit authentication or explicit tenancy.
type requestInterceptor struct {
	tokens TokenSource
	tenant string
}

func (i requestInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		setTenant(req.Any(), i.tenant)
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
		wrapped := &requestClientConn{StreamingClientConn: conn, tenant: i.tenant}
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
	tenant string
	err    error
}

func (c *requestClientConn) Send(message any) error {
	if c.err != nil {
		return c.err
	}
	setTenant(message, c.tenant)
	return c.StreamingClientConn.Send(message)
}

func (c *requestClientConn) Receive(message any) error {
	if c.err != nil {
		return c.err
	}
	return c.StreamingClientConn.Receive(message)
}

func setTenant(message any, tenant string) {
	if tenant == "" {
		return
	}
	value, ok := message.(proto.Message)
	if !ok {
		return
	}
	reflected := value.ProtoReflect()
	field := reflected.Descriptor().Fields().ByName(protoreflect.Name("tenant_id"))
	if field != nil && field.Kind() == protoreflect.StringKind && reflected.Get(field).String() == "" {
		reflected.Set(field, protoreflect.ValueOfString(tenant))
	}
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
