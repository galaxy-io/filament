package remote

import (
	"context"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Discover runs connector discovery on the deployment, against a saved
// connection when the request names one.
func (t *Target) Discover(ctx context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	message := &ingestionv1.DiscoverResourcesRequest{
		Connector: request.Connector,
		Refresh:   request.Refresh,
	}
	config, err := configStruct(request.Config)
	if err != nil {
		return model.ResourceList{}, err
	}
	message.Config = config
	if request.Source != "" {
		connection, err := t.findConnection(ctx, "source", request.Source)
		if err != nil {
			return model.ResourceList{}, err
		}
		message.ConnectionId = connection.GetId()
	}
	response, err := t.client.DiscoverResources(ctx, connect.NewRequest(message))
	if err != nil {
		return model.ResourceList{}, t.rpcError(err)
	}
	resources := response.Msg.GetResources()
	list := model.ResourceList{Source: request.Source, Items: make([]model.ResourceSummary, 0, len(resources))}
	for _, resource := range resources {
		list.Items = append(list.Items, model.ResourceSummary{
			Name:        resource.GetName(),
			DisplayName: resource.GetDisplayName(),
			Selectable:  resource.GetIsSelectable(),
			PrimaryKey:  resource.GetPrimaryKey(),
		})
	}
	return list, nil
}
