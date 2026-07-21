package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreateConnection separates schema-declared secret fields from ordinary
// config, writes their values to the configured secret provider, and persists
// only opaque references alongside the non-secret config.
func (a *Server) CreateConnection(ctx context.Context, req *connect.Request[ingestionv1.CreateConnectionRequest]) (*connect.Response[ingestionv1.CreateConnectionResponse], error) {
	schema, err := a.schemaFor(req.Msg.GetKind(), req.Msg.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	cfg := structMap(req.Msg.GetConfig())
	if err := validateConnectionConfig(schema, cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id := uuid.NewString()
	tenant := defaultTenant(req.Msg.GetTenantId())
	refs := cloneStrings(req.Msg.GetSecretRefs())
	if err := validateSecretRefTenant(refs, tenant); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	written, err := a.storeSecretFields(ctx, schema, tenant, id, 1, cfg, refs)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	config, err := structpb.NewStruct(cfg)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	conn, err := a.store.CreateConnection(ctx, filament.Connection{
		ID: id, Tenant: tenant, Kind: connectionKindFromProto(req.Msg.GetKind()), Name: req.Msg.GetName(),
		Connector: req.Msg.GetConnector(), Config: config.AsMap(), SecretRefs: refs,
	})
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.CreateConnectionResponse{Connection: connectionToProto(conn)}), nil
}

// UpdateConnection applies changes to an existing connection, enforcing optimistic versioning.
func (a *Server) UpdateConnection(ctx context.Context, req *connect.Request[ingestionv1.UpdateConnectionRequest]) (*connect.Response[ingestionv1.UpdateConnectionResponse], error) {
	in := req.Msg.GetConnection()
	if in == nil || in.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("connection.id is required"))
	}
	stored, err := a.store.LoadConnection(ctx, in.GetId())
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if stored.Version != in.GetVersion() {
		return nil, connect.NewError(connect.CodeAborted, fmt.Errorf("connection %q version conflict: have %d, got %d", in.GetId(), stored.Version, in.GetVersion()))
	}
	schema, err := a.schemaFor(in.GetKind(), in.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	cfg := structMap(in.GetConfig())
	if err := validateConnectionConfig(schema, cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	refs := cloneStrings(in.GetSecretRefs())
	if err := validateSecretRefTenant(refs, stored.Tenant); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// A newly submitted value gets a versioned ref. The old value remains active
	// until the optimistic connection update succeeds.
	written, err := a.storeSecretFields(ctx, schema, stored.Tenant, in.GetId(), in.GetVersion()+1, cfg, refs)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	config, err := structpb.NewStruct(cfg)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	next := connectionFromProto(proto.Clone(in).(*ingestionv1.Connection))
	next.Config, next.SecretRefs = config.AsMap(), refs
	if next.Tenant == "" {
		next.Tenant = stored.Tenant
	}
	next, err = a.store.UpdateConnection(ctx, next)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeAborted, err)
	}
	a.deleteReplacedSecretRefs(ctx, stored.SecretRefs, next.SecretRefs)
	return connect.NewResponse(&ingestionv1.UpdateConnectionResponse{Connection: connectionToProto(next)}), nil
}

// GetConnection returns the connection with the requested ID.
func (a *Server) GetConnection(ctx context.Context, req *connect.Request[ingestionv1.GetConnectionRequest]) (*connect.Response[ingestionv1.GetConnectionResponse], error) {
	conn, err := a.store.LoadConnection(ctx, req.Msg.GetId())
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.GetConnectionResponse{Connection: connectionToProto(conn)}), nil
}

// ListConnections returns connections matching the request's tenant and kind filter.
func (a *Server) ListConnections(ctx context.Context, req *connect.Request[ingestionv1.ListConnectionsRequest]) (*connect.Response[ingestionv1.ListConnectionsResponse], error) {
	connections, err := a.store.ListConnections(ctx, filament.ConnectionFilter{Tenant: req.Msg.GetTenantId(), Kind: connectionKindFromProto(req.Msg.GetKind())})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*ingestionv1.Connection, len(connections))
	for i, c := range connections {
		out[i] = connectionToProto(c)
	}
	return connect.NewResponse(&ingestionv1.ListConnectionsResponse{Connections: out}), nil
}

// DeleteConnection removes the connection with the requested ID.
func (a *Server) DeleteConnection(ctx context.Context, req *connect.Request[ingestionv1.DeleteConnectionRequest]) (*connect.Response[ingestionv1.DeleteConnectionResponse], error) {
	id := req.Msg.GetId()
	conn, loadErr := a.store.LoadConnection(ctx, id)
	if loadErr != nil && !errors.Is(loadErr, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeInternal, loadErr)
	}

	pipelines, err := a.store.ListPipelines(ctx, "")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, pipeline := range pipelines {
		version, err := a.store.LoadPipelineVersion(ctx, pipeline.GetId(), 0)
		if errors.Is(err, filament.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range version.GetNodes() {
			if node.GetConnectionId() == id {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %q is in use by pipeline %q", id, pipeline.GetId()))
			}
		}
	}
	if err := a.store.DeleteConnection(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if loadErr == nil {
		a.deleteSecretRefs(ctx, mapValues(conn.SecretRefs))
	}
	return connect.NewResponse(&ingestionv1.DeleteConnectionResponse{}), nil
}

func (a *Server) storeSecretFields(ctx context.Context, schema filament.ConfigSchema, tenant, id string, version int64, cfg map[string]any, refs map[string]string) ([]string, error) {
	var written []string
	for _, field := range schema.Fields {
		if field.Type != filament.FieldSecret && !field.Secret {
			continue
		}
		value, present := cfg[field.Name]
		delete(cfg, field.Name) // plaintext must never reach the connection store
		if !present {
			continue
		}
		s, ok := value.(string)
		if !ok {
			a.deleteSecretRefs(ctx, written)
			return nil, fmt.Errorf("secret field %q must be a string", field.Name)
		}
		if a.secrets == nil {
			a.deleteSecretRefs(ctx, written)
			return nil, fmt.Errorf("secret field %q supplied but no secret provider is configured", field.Name)
		}
		ref := filament.ConnectionSecretRef(tenant, id, field.Name, version)
		if err := a.secrets.Write(ctx, ref, filament.Secret{Value: []byte(s), Meta: map[string]string{"tenant": tenant, "connection": id, "field": field.Name}}); err != nil {
			a.deleteSecretRefs(ctx, written)
			return nil, fmt.Errorf("store secret field %q: %w", field.Name, err)
		}
		refs[field.Name] = ref
		written = append(written, ref)
	}
	return written, nil
}

// resolveConnectionSecrets reads each of the connection's secret refs and
// injects the plaintext value into cfg under the mapped field. Every ref is
// tenant-scoped first, so a connection can never read another tenant's secrets.
func (a *Server) resolveConnectionSecrets(ctx context.Context, conn filament.Connection, cfg map[string]any) error {
	if len(conn.SecretRefs) == 0 {
		return nil
	}
	if a.secrets == nil {
		return fmt.Errorf("connection %q has secret refs but no secret provider is configured", conn.ID)
	}
	for field, ref := range conn.SecretRefs {
		if err := filament.ValidateConnectionSecretRef(ref, filament.TenantID(conn.Tenant)); err != nil {
			return fmt.Errorf("resolve secret for field %q: %w", field, err)
		}
		secret, err := a.secrets.Read(ctx, ref)
		if err != nil {
			return fmt.Errorf("resolve secret %q for field %q: %w", ref, field, err)
		}
		cfg[field] = string(secret.Value)
	}
	return nil
}

func (a *Server) deleteReplacedSecretRefs(ctx context.Context, old, next map[string]string) {
	for field, ref := range old {
		if next[field] != ref {
			a.deleteSecretRefs(ctx, []string{ref})
		}
	}
}

func (a *Server) deleteSecretRefs(ctx context.Context, refs []string) {
	if a.secrets == nil {
		return
	}
	for _, ref := range refs {
		// Only ever delete refs this server minted; never a caller-supplied ref
		// that might point at an env var or another store's key.
		if strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
			_ = a.secrets.Delete(ctx, ref)
		}
	}
}

// validateSecretRefTenant rejects any caller-supplied ref in the
// connection-managed namespace that names a different tenant, so a connection
// can never reference another tenant's secrets.
func validateSecretRefTenant(refs map[string]string, tenant string) error {
	for field, ref := range refs {
		if err := filament.ValidateConnectionSecretRef(ref, filament.TenantID(tenant)); err != nil {
			return fmt.Errorf("secret ref for field %q: %w", field, err)
		}
	}
	return nil
}

func cloneStrings(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mapValues(in map[string]string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	return out
}
