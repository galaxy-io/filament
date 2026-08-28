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
	"github.com/galaxy-io/filament/internal/compile"
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
	refs := cloneStrings(req.Msg.GetSecretRefs())
	tenant := defaultTenant(req.Msg.GetTenantId())
	canonicalizeConnectionConfig(schema, cfg, refs)
	if err := validateSecretRefTenant(refs, tenant); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := compile.ValidateConnectionConfig(schema, cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	effective := map[string]any{}
	if err := a.resolveConnectionSecrets(ctx, filament.Connection{ID: "new", Tenant: tenant, SecretRefs: refs}, effective); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	effective = overlayConfig(effective, cfg)
	if err := validateConfigSchema(schema, filament.NewConfig(effective)); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.validateConnectionConnectorConfig(req.Msg.GetKind(), req.Msg.GetConnector(), filament.NewConfig(effective)); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id := uuid.NewString()
	written, err := a.storeSecretFields(ctx, schema, tenant, id, 1, cfg, refs, false)
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
	return connect.NewResponse(&ingestionv1.CreateConnectionResponse{Connection: a.connectionForResponse(conn)}), nil
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
	if stored.DeletedAt != 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %q is deleted", in.GetId()))
	}
	if stored.Version != in.GetVersion() {
		return nil, connect.NewError(connect.CodeAborted, fmt.Errorf("connection %q version conflict: have %d, got %d", in.GetId(), stored.Version, in.GetVersion()))
	}
	if stored.Connector != in.GetConnector() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("connection %q connector cannot change from %q to %q", in.GetId(), stored.Connector, in.GetConnector()))
	}
	if stored.Kind != connectionKindFromProto(in.GetKind()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("connection %q kind cannot change", in.GetId()))
	}
	schema, err := a.schemaFor(in.GetKind(), in.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	cfg := structMap(in.GetConfig())
	refs := cloneStrings(in.GetSecretRefs())
	canonicalizeConnectionConfig(schema, cfg, refs)
	if err := validateSecretRefTenant(refs, stored.Tenant); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := compile.ValidateConnectionConfig(schema, cfg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	effective := cloneConfigMap(stored.Config)
	if err := a.resolveConnectionSecrets(ctx, filament.Connection{ID: stored.ID, Tenant: stored.Tenant, SecretRefs: refs}, effective); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	effective = overlayConfig(effective, cfg)
	canonicalizeConnectionConfig(schema, effective, cloneStrings(refs))
	if err := validateConfigSchema(schema, filament.NewConfig(effective)); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.validateConnectionConnectorConfig(in.GetKind(), in.GetConnector(), filament.NewConfig(effective)); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if stored.Kind == filament.ConnectorKindSource {
		source, err := a.sources.Resolve(stored.Connector)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		before := filament.ReplicationOf(source, filament.NewConfig(stored.Config))
		after := filament.ReplicationOf(source, filament.NewConfig(cfg))
		if before != after {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("connection replication mode is immutable; create a new connection to change from %s to %s", before, after))
		}
	}
	// A newly submitted value gets a versioned ref. The old value remains active
	// until the optimistic connection update succeeds.
	written, err := a.storeSecretFields(ctx, schema, stored.Tenant, in.GetId(), in.GetVersion()+1, cfg, refs, true)
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
	return connect.NewResponse(&ingestionv1.UpdateConnectionResponse{Connection: a.connectionForResponse(next)}), nil
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
	return connect.NewResponse(&ingestionv1.GetConnectionResponse{Connection: a.connectionForResponse(conn)}), nil
}

// ListConnections returns connections matching the request's tenant and kind filter.
func (a *Server) ListConnections(ctx context.Context, req *connect.Request[ingestionv1.ListConnectionsRequest]) (*connect.Response[ingestionv1.ListConnectionsResponse], error) {
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME:       "name",
		ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at",
		ingestionv1.SortBy_SORT_BY_UPDATED_AT: "updated_at",
	}, "id", false)
	if err != nil {
		return nil, err
	}
	connections, total, err := a.store.ListConnections(ctx, filament.ConnectionFilter{
		Tenant: req.Msg.GetTenantId(), Kind: connectionKindFromProto(req.Msg.GetKind()),
		IncludeDeleted: req.Msg.GetIncludeDeleted(), ListOptions: options,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*ingestionv1.Connection, len(connections))
	for i, c := range connections {
		out[i] = a.connectionForResponse(c)
	}
	return connect.NewResponse(&ingestionv1.ListConnectionsResponse{Connections: out, Pagination: paginationOf(req.Msg.GetPagination(), options, total)}), nil
}

func (a *Server) connectionForResponse(conn filament.Connection) *ingestionv1.Connection {
	conn.Config = cloneConfigMap(conn.Config)
	if schema, err := a.schemaFor(connectionKindToProto(conn.Kind), conn.Connector); err == nil {
		canonicalizeConnectionConfig(schema, conn.Config, cloneStrings(conn.SecretRefs))
	}
	out := connectionToProto(conn)
	if conn.Kind != filament.ConnectorKindSource {
		return out
	}
	source, err := a.sources.Resolve(conn.Connector)
	if err != nil {
		return out
	}
	out.Replication = replicationToProto(filament.ReplicationOf(source, filament.NewConfig(conn.Config)))
	return out
}

// DeleteConnection removes the connection with the requested ID.
func (a *Server) DeleteConnection(ctx context.Context, req *connect.Request[ingestionv1.DeleteConnectionRequest]) (*connect.Response[ingestionv1.DeleteConnectionResponse], error) {
	id := req.Msg.GetId()
	conn, loadErr := a.store.LoadConnection(ctx, id)
	if loadErr != nil && !errors.Is(loadErr, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeInternal, loadErr)
	}

	pipelines, _, err := a.store.ListPipelines(ctx, filament.PipelineFilter{})
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
		for _, node := range version.GetGraph().GetNodes() {
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

func (a *Server) storeSecretFields(ctx context.Context, schema filament.ConfigSchema, tenant, id string, version int64, cfg map[string]any, refs map[string]string, isUpdate bool) ([]string, error) {
	fields, err := extractSecretFields(schema.Fields, cfg, "", isUpdate)
	if err != nil {
		return nil, err
	}
	if len(fields) > 0 && a.secrets == nil {
		return nil, fmt.Errorf("secret field %q supplied but no secret provider is configured", fields[0].path)
	}

	written := make([]string, 0, len(fields))
	for _, field := range fields {
		ref := filament.ConnectionSecretRef(tenant, id, field.path, version)
		secret := filament.Secret{
			Tenant: filament.TenantID(tenant),
			Value:  []byte(field.value),
			Meta:   map[string]string{"tenant": tenant, "connection": id, "field": field.path},
		}
		if err := a.secrets.Write(ctx, ref, secret); err != nil {
			a.deleteSecretRefs(ctx, written)
			return nil, fmt.Errorf("store secret field %q: %w", field.path, err)
		}
		refs[field.path] = ref
		written = append(written, ref)
	}
	return written, nil
}

type extractedSecretField struct {
	path  string
	value string
}

// extractSecretFields removes schema-declared secret values from config and
// returns their dotted paths for storage. Objects left empty by extraction are
// removed as well, so secret-only containers are not persisted. On update, a
// blank value means not provided and the existing ref is kept; on create
// there is no existing ref, so a blank value on a required field is an error.
func extractSecretFields(schema []filament.ConfigField, cfg map[string]any, parent string, isUpdate bool) ([]extractedSecretField, error) {
	var secrets []extractedSecretField
	for _, field := range schema {
		path := joinConfigPath(parent, field.Name)
		if field.Type == filament.FieldSecret || field.Secret {
			value, present := cfg[field.Name]
			delete(cfg, field.Name) // plaintext must never reach the connection store
			if !present {
				continue
			}
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("secret field %q must be a string", path)
			}
			if s == "" {
				if !isUpdate && field.Required {
					return nil, fmt.Errorf("secret field %q is required", path)
				}
				continue
			}
			secrets = append(secrets, extractedSecretField{path: path, value: s})
			continue
		}

		nested, ok := cfg[field.Name].(map[string]any)
		if !ok || len(field.Fields) == 0 {
			continue
		}
		extracted, err := extractSecretFields(field.Fields, nested, path, isUpdate)
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, extracted...)
		if len(nested) == 0 {
			delete(cfg, field.Name)
		}
	}
	return secrets, nil
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
		setConfigPath(cfg, field, string(secret.Value))
	}
	return nil
}

// loadConnectionForTenant loads the connection by id and rejects it if tenant
// does not match, so a caller can never merge or resolve secrets from another
// tenant's connection just by naming its id.
func (a *Server) loadConnectionForTenant(ctx context.Context, id, tenant string) (filament.Connection, error) {
	conn, err := a.store.LoadConnection(ctx, id)
	if err != nil {
		return filament.Connection{}, err
	}
	if conn.Tenant != defaultTenant(tenant) {
		return filament.Connection{}, filament.ErrNotFound
	}
	return conn, nil
}

// overlayConfig recursively merges overlay over base. A blank string at any
// depth means "not supplied" and defers to base, so an untouched secret field
// never clobbers a resolved value with "" inside a nested config object.
func overlayConfig(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = cloneConfigValue(v)
	}
	for k, v := range overlay {
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		if nestedOverlay, ok := v.(map[string]any); ok {
			if nestedBase, ok := out[k].(map[string]any); ok {
				out[k] = overlayConfig(nestedBase, nestedOverlay)
				continue
			}
			out[k] = overlayConfig(nil, nestedOverlay)
			continue
		}
		out[k] = cloneConfigValue(v)
	}
	return out
}

func cloneConfigMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneConfigValue(value)
	}
	return out
}

func cloneConfigValue(value any) any {
	if nested, ok := value.(map[string]any); ok {
		return cloneConfigMap(nested)
	}
	return value
}

// canonicalizeConnectionConfig materializes the fields-first database selector
// and removes known fields from inactive conditional branches. A missing
// selector with an existing DSN remains URL mode for legacy connections.
func canonicalizeConnectionConfig(schema filament.ConfigSchema, cfg map[string]any, refs map[string]string) {
	if hasConfigField(schema.Fields, "connection_method") {
		if method, _ := cfg["connection_method"].(string); method == "" {
			if dsn, _ := cfg["dsn"].(string); dsn != "" || refs["dsn"] != "" {
				cfg["connection_method"] = "url"
			} else {
				cfg["connection_method"] = "fields"
			}
		}
	}
	pruneInactiveFields(schema.Fields, cfg, refs, "")
}

func hasConfigField(fields []filament.ConfigField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

func pruneInactiveFields(fields []filament.ConfigField, cfg map[string]any, refs map[string]string, parent string) {
	config := filament.NewConfig(cfg)
	// A name may be declared once per conditional branch (e.g. one uri field
	// per catalog provider); it is inactive only when no declaration is visible.
	visible := make(map[string]bool, len(fields))
	for _, field := range fields {
		if fieldIsVisible(field, config) {
			visible[field.Name] = true
		}
	}
	for _, field := range fields {
		path := joinConfigPath(parent, field.Name)
		if !visible[field.Name] {
			delete(cfg, field.Name)
			delete(refs, path)
			continue
		}
		if !fieldIsVisible(field, config) {
			continue
		}
		nested, ok := cfg[field.Name].(map[string]any)
		if ok && len(field.Fields) > 0 {
			pruneInactiveFields(field.Fields, nested, refs, path)
		}
	}
}

func joinConfigPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}

func setConfigPath(cfg map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := cfg
	for _, part := range parts[:len(parts)-1] {
		nested, ok := current[part].(map[string]any)
		if !ok {
			nested = make(map[string]any)
			current[part] = nested
		}
		current = nested
	}
	current[parts[len(parts)-1]] = value
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
