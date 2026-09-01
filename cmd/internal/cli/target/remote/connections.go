package remote

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// ListConnections returns named connections with deployment-owned metadata.
func (t *Target) ListConnections(ctx context.Context, kind string) ([]model.NamedConnection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return nil, err
	}
	response, err := t.client.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{Kind: protoKind}))
	if err != nil {
		return nil, t.rpcError(err)
	}
	connections := make([]model.NamedConnection, 0, len(response.Msg.GetConnections()))
	for _, item := range response.Msg.GetConnections() {
		connections = append(connections, model.NamedConnection{
			Kind: kind, Name: item.GetName(), Connection: connectionFromProto(item),
		})
	}
	return connections, nil
}

// GetConnection returns one connection by name.
func (t *Target) GetConnection(ctx context.Context, kind, name string) (model.Connection, error) {
	item, err := t.findConnection(ctx, kind, name)
	if err != nil {
		return model.Connection{}, err
	}
	return connectionFromProto(item), nil
}

// CreateConnection creates a named connection on the deployment.
func (t *Target) CreateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return model.Connection{}, err
	}
	config, err := configStruct(connection.Config)
	if err != nil {
		return model.Connection{}, err
	}
	response, err := t.client.CreateConnection(ctx, connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: protoKind, Name: name, Connector: connection.Type, Config: config,
	}))
	if err != nil {
		return model.Connection{}, t.rpcError(err)
	}
	return connectionFromProto(response.Msg.GetConnection()), nil
}

// UpdateConnection replaces a connection's configuration under its read
// revision; the deployment rejects a stale one.
func (t *Target) UpdateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return model.Connection{}, err
	}
	id := connection.Metadata.ID
	if id == "" {
		existing, findErr := t.findConnection(ctx, kind, name)
		if findErr != nil {
			return model.Connection{}, findErr
		}
		id = existing.GetId()
	}
	version, err := strconv.ParseInt(connection.Metadata.Revision, 10, 64)
	if err != nil {
		return model.Connection{}, fmt.Errorf("%s %q carries no read revision", kind, name)
	}
	config, err := configStruct(connection.Config)
	if err != nil {
		return model.Connection{}, err
	}
	response, err := t.client.UpdateConnection(ctx, connect.NewRequest(&ingestionv1.UpdateConnectionRequest{
		Connection: &ingestionv1.Connection{
			Id: id, Kind: protoKind, Name: name, Connector: connection.Type,
			Config: config, Version: version,
		},
	}))
	if err != nil {
		return model.Connection{}, t.rpcError(err)
	}
	return connectionFromProto(response.Msg.GetConnection()), nil
}

// DeleteConnection removes a connection by name.
func (t *Target) DeleteConnection(ctx context.Context, kind, name string, metadata model.EntityMetadata) error {
	id := metadata.ID
	if id == "" {
		existing, err := t.findConnection(ctx, kind, name)
		if err != nil {
			return err
		}
		id = existing.GetId()
	}
	_, err := t.client.DeleteConnection(ctx, connect.NewRequest(&ingestionv1.DeleteConnectionRequest{Id: id}))
	return t.rpcError(err)
}

func (t *Target) findConnection(ctx context.Context, kind, name string) (*ingestionv1.Connection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return nil, err
	}
	response, err := t.client.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{Kind: protoKind}))
	if err != nil {
		return nil, t.rpcError(err)
	}
	for _, item := range response.Msg.GetConnections() {
		if item.GetName() == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("%s %q does not exist", kind, name)
}

// connectionNamesByID maps every connection id to its name for graph reads.
func (t *Target) connectionNamesByID(ctx context.Context) (map[string]string, error) {
	response, err := t.client.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{}))
	if err != nil {
		return nil, t.rpcError(err)
	}
	names := make(map[string]string, len(response.Msg.GetConnections()))
	for _, item := range response.Msg.GetConnections() {
		names[item.GetId()] = item.GetName()
	}
	return names, nil
}

func connectionFromProto(item *ingestionv1.Connection) model.Connection {
	return model.Connection{
		Metadata: model.EntityMetadata{ID: item.GetId(), Revision: revisionOf(item.GetVersion())},
		Type:     item.GetConnector(),
		Config:   configMap(item.GetConfig()),
	}
}
