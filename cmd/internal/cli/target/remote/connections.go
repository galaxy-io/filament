package remote

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// ListConnections returns named connections with deployment-owned metadata.
func (t *Target) ListConnections(ctx context.Context, kind string) ([]model.NamedConnection, error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return nil, err
	}
	items, err := t.allConnections(ctx, protoKind)
	if err != nil {
		return nil, err
	}
	connections := make([]model.NamedConnection, 0, len(items))
	for _, item := range items {
		connections = append(connections, model.NamedConnection{
			Kind: kind, Name: item.GetName(), Connection: connectionFromProto(item),
		})
	}
	return connections, nil
}

// ListConnectionsPage returns one deployment-native cursor page.
func (t *Target) ListConnectionsPage(ctx context.Context, kind string, request model.PageRequest) (model.Page[model.NamedConnection], error) {
	protoKind, err := connectorKind(kind)
	if err != nil {
		return model.Page[model.NamedConnection]{}, err
	}
	items, pagination, err := t.connectionPage(ctx, protoKind, request)
	if err != nil {
		return model.Page[model.NamedConnection]{}, err
	}
	page := model.Page[model.NamedConnection]{
		Items: make([]model.NamedConnection, 0, len(items)), Total: int(pagination.GetTotal()),
		NextCursor: pagination.GetNextCursor(), PreviousCursor: pagination.GetPreviousCursor(),
	}
	for _, item := range items {
		page.Items = append(page.Items, model.NamedConnection{
			Kind: kind, Name: item.GetName(), Connection: connectionFromProto(item),
		})
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
		SecretRefs: cloneStrings(connection.SecretRefs),
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
	config, err := configStruct(cliapp.ConfigWithoutSecretValues(connection.Config, connection.SecretRefs))
	if err != nil {
		return model.Connection{}, err
	}
	response, err := t.client.UpdateConnection(ctx, connect.NewRequest(&ingestionv1.UpdateConnectionRequest{
		Connection: &ingestionv1.Connection{
			Id: id, Kind: protoKind, Name: name, Connector: connection.Type,
			Config: config, SecretRefs: cloneStrings(connection.SecretRefs), Version: version,
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
	items, err := t.allConnections(ctx, protoKind)
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

func (t *Target) allConnections(ctx context.Context, kind ingestionv1.ConnectorKind) ([]*ingestionv1.Connection, error) {
	var items []*ingestionv1.Connection
	cursor := ""
	for {
		page, pagination, err := t.connectionPage(ctx, kind, model.PageRequest{PageSize: internalPageSize, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		cursor = pagination.GetNextCursor()
		if cursor == "" {
			return items, nil
		}
	}
}

func (t *Target) connectionPage(
	ctx context.Context,
	kind ingestionv1.ConnectorKind,
	request model.PageRequest,
) ([]*ingestionv1.Connection, *ingestionv1.PaginationResponse, error) {
	response, err := t.client.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{
		Kind: kind, Pagination: paginationRequest(request.PageSize, request.Cursor),
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
		SecretRefs: cloneStrings(item.GetSecretRefs()),
	}
}

func cloneStrings(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func replicationString(mode ingestionv1.ReplicationMode) string {
	switch mode {
	case ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD:
		return "standard"
	case ingestionv1.ReplicationMode_REPLICATION_MODE_CDC:
		return "cdc"
	case ingestionv1.ReplicationMode_REPLICATION_MODE_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}
