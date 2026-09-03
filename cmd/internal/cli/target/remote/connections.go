package remote

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// ListConnections returns one page of connections of one kind, with the
// deployment's own cursors and total.
func (t *Target) ListConnections(ctx context.Context, kind string, request model.PageRequest) (model.Page[model.NamedConnection], error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return model.Page[model.NamedConnection]{}, err
	}
	items, info, err := t.connectionPage(ctx, protoKind, "", request)
	if err != nil {
		return model.Page[model.NamedConnection]{}, err
	}
	page := model.Page[model.NamedConnection]{Items: make([]model.NamedConnection, 0, len(items)), PageInfo: pageInfo(info)}
	for _, item := range items {
		page.Items = append(page.Items, model.NamedConnection{Kind: kind, Name: item.GetName(), Connection: connectionFromProto(item)})
	}
	return page, nil
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
	config, err := configStruct(cliapp.ConfigWithoutSecretValues(connection.Config, connection.SecretRefs))
	if err != nil {
		return model.Connection{}, err
	}
	response, err := t.client.CreateConnection(ctx, connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: protoKind, Name: name, Connector: connection.Type, Config: config,
		SecretRefs: maps.Clone(connection.SecretRefs),
	}))
	if err != nil {
		return model.Connection{}, t.rpcError(err)
	}
	return connectionFromProto(response.Msg.GetConnection()), nil
}

// UpdateConnection replaces a connection's configuration under the revision
// it was read at; the deployment rejects a stale one.
func (t *Target) UpdateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return model.Connection{}, err
	}
	id, err := t.connectionID(ctx, kind, name, connection.Metadata)
	if err != nil {
		return model.Connection{}, err
	}
	version, err := strconv.ParseInt(connection.Metadata.Revision, 10, 64)
	if err != nil {
		return model.Connection{}, fmt.Errorf("%s %q carries no read revision", kind, name)
	}
	config, err := configStruct(cliapp.ConfigWithoutSecretValues(connection.Config, connection.SecretRefs))
	if err != nil {
		return model.Connection{}, err
	}
	response, err := t.client.UpdateConnection(ctx, connect.NewRequest(&ingestionv1.UpdateConnectionRequest{
		Connection: &ingestionv1.Connection{
			Id: id, Kind: protoKind, Name: name, Connector: connection.Type,
			Config: config, SecretRefs: maps.Clone(connection.SecretRefs), Version: version,
		},
	}))
	if err != nil {
		return model.Connection{}, t.rpcError(err)
	}
	return connectionFromProto(response.Msg.GetConnection()), nil
}

// DeleteConnection removes a connection by name.
func (t *Target) DeleteConnection(ctx context.Context, kind, name string, metadata model.EntityMetadata) error {
	id, err := t.connectionID(ctx, kind, name, metadata)
	if err != nil {
		return err
	}
	_, err = t.client.DeleteConnection(ctx, connect.NewRequest(&ingestionv1.DeleteConnectionRequest{Id: id}))
	return t.rpcError(err)
}

// connectionID prefers the id the caller read; a bare name is resolved.
func (t *Target) connectionID(ctx context.Context, kind, name string, metadata model.EntityMetadata) (string, error) {
	if metadata.ID != "" {
		return metadata.ID, nil
	}
	item, err := t.findConnection(ctx, kind, name)
	if err != nil {
		return "", err
	}
	return item.GetId(), nil
}

// findConnection resolves a name, narrowed server-side by the substring
// search and matched exactly here.
func (t *Target) findConnection(ctx context.Context, kind, name string) (*ingestionv1.Connection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return nil, err
	}
	items, err := t.connections(ctx, protoKind, name)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.GetName() == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("%s %q does not exist", kind, name)
}

func (t *Target) connections(ctx context.Context, kind ingestionv1.ConnectorKind, search string) ([]*ingestionv1.Connection, error) {
	return drain(func(cursor string) ([]*ingestionv1.Connection, *ingestionv1.PaginationResponse, error) {
		return t.connectionPage(ctx, kind, search, model.PageRequest{PageSize: pageSize, Cursor: cursor})
	})
}

func (t *Target) connectionPage(ctx context.Context, kind ingestionv1.ConnectorKind, search string, request model.PageRequest) ([]*ingestionv1.Connection, *ingestionv1.PaginationResponse, error) {
	response, err := t.client.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{
		Kind: kind, Search: search, Pagination: pagination(request),
	}))
	if err != nil {
		return nil, nil, t.rpcError(err)
	}
	return response.Msg.GetConnections(), response.Msg.GetPagination(), nil
}

func connectionFromProto(item *ingestionv1.Connection) model.Connection {
	return model.Connection{
		Metadata: model.EntityMetadata{ID: item.GetId(), Revision: revisionOf(item.GetVersion())},
		Info: model.ConnectionInfo{
			Replication: replicationString(item.GetReplication()),
			CreatedAt:   timeFromMillis(item.GetCreatedAt()),
			UpdatedAt:   timeFromMillis(item.GetUpdatedAt()),
		},
		Type:       item.GetConnector(),
		Config:     configMap(item.GetConfig()),
		SecretRefs: maps.Clone(item.GetSecretRefs()),
	}
}

func replicationString(mode ingestionv1.ReplicationMode) string {
	switch mode {
	case ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD:
		return "standard"
	case ingestionv1.ReplicationMode_REPLICATION_MODE_CDC:
		return model.SyncModeCDC
	default:
		return ""
	}
}
