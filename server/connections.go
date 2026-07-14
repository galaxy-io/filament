package server

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreateConnection validates the config against the connector's schema and
// stores a new connection at version 1.
func (a *Server) CreateConnection(_ context.Context, req *connect.Request[ingestionv1.CreateConnectionRequest]) (*connect.Response[ingestionv1.CreateConnectionResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	schema, err := a.schemaFor(req.Msg.GetKind(), req.Msg.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateConnectionConfig(schema, structMap(req.Msg.GetConfig())); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	a.nextConnID++
	id := fmt.Sprintf("conn-%d", a.nextConnID)
	conn := &ingestionv1.Connection{
		Id:         id,
		Tenant:     defaultTenant(req.Msg.GetTenant()),
		Kind:       req.Msg.GetKind(),
		Name:       req.Msg.GetName(),
		Connector:  req.Msg.GetConnector(),
		Config:     req.Msg.GetConfig(),
		SecretRefs: req.Msg.GetSecretRefs(),
		Version:    1,
	}
	a.connections[id] = proto.Clone(conn).(*ingestionv1.Connection)
	return connect.NewResponse(&ingestionv1.CreateConnectionResponse{Connection: conn}), nil
}

// UpdateConnection replaces a stored connection if the request's version matches
// the stored one, returning CodeAborted on a version conflict.
func (a *Server) UpdateConnection(_ context.Context, req *connect.Request[ingestionv1.UpdateConnectionRequest]) (*connect.Response[ingestionv1.UpdateConnectionResponse], error) {
	conn := req.Msg.GetConnection()
	if conn == nil || conn.GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("connection.id is required"))
	}
	schema, err := a.schemaFor(conn.GetKind(), conn.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateConnectionConfig(schema, structMap(conn.GetConfig())); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	stored := a.connections[conn.GetId()]
	if stored == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("connection %q not found", conn.GetId()))
	}
	if stored.GetVersion() != conn.GetVersion() {
		return nil, connect.NewError(connect.CodeAborted, fmt.Errorf("connection %q version conflict: have %d, got %d", conn.GetId(), stored.GetVersion(), conn.GetVersion()))
	}
	next := proto.Clone(conn).(*ingestionv1.Connection)
	next.Version++
	if next.Tenant == "" {
		next.Tenant = stored.GetTenant()
	}
	a.connections[next.GetId()] = next
	return connect.NewResponse(&ingestionv1.UpdateConnectionResponse{Connection: proto.Clone(next).(*ingestionv1.Connection)}), nil
}

// GetConnection returns the connection with the given id.
func (a *Server) GetConnection(_ context.Context, req *connect.Request[ingestionv1.GetConnectionRequest]) (*connect.Response[ingestionv1.GetConnectionResponse], error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	conn := a.connections[req.Msg.GetId()]
	if conn == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("connection %q not found", req.Msg.GetId()))
	}
	return connect.NewResponse(&ingestionv1.GetConnectionResponse{Connection: proto.Clone(conn).(*ingestionv1.Connection)}), nil
}

// ListConnections returns the stored connections sorted by id, optionally
// filtered by tenant and kind.
func (a *Server) ListConnections(_ context.Context, req *connect.Request[ingestionv1.ListConnectionsRequest]) (*connect.Response[ingestionv1.ListConnectionsResponse], error) {
	tenant := req.Msg.GetTenant()
	kind := req.Msg.GetKind()

	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []*ingestionv1.Connection
	for _, conn := range a.connections {
		if tenant != "" && conn.GetTenant() != tenant {
			continue
		}
		if kind != ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED && conn.GetKind() != kind {
			continue
		}
		out = append(out, proto.Clone(conn).(*ingestionv1.Connection))
	}
	slices.SortFunc(out, func(x, y *ingestionv1.Connection) int {
		return strings.Compare(x.GetId(), y.GetId())
	})
	return connect.NewResponse(&ingestionv1.ListConnectionsResponse{Connections: out}), nil
}

// DeleteConnection removes a connection, refusing while any pipeline node still
// references it.
func (a *Server) DeleteConnection(_ context.Context, req *connect.Request[ingestionv1.DeleteConnectionRequest]) (*connect.Response[ingestionv1.DeleteConnectionResponse], error) {
	id := req.Msg.GetId()
	a.mu.Lock()
	defer a.mu.Unlock()

	// Block delete while any pipeline node still references this connection.
	for _, pipeline := range a.pipelines {
		for _, node := range pipeline.GetNodes() {
			if node.GetConnectionId() == id {
				return nil, connect.NewError(connect.CodeFailedPrecondition,
					fmt.Errorf("connection %q is in use by pipeline %q", id, pipeline.GetId()))
			}
		}
	}
	delete(a.connections, id)
	return connect.NewResponse(&ingestionv1.DeleteConnectionResponse{}), nil
}
