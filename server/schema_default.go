package server

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/naming"
)

// defaultSinkSchemas fills each sink node's declared schema field (per
// SinkSpec.SchemaField) with the normalized name of its upstream source
// connection. Explicit values win; a node with zero or multiple upstream
// source connections, or any resolution failure, is left untouched.
func (a *Server) defaultSinkSchemas(ctx context.Context, tenant filament.TenantID, nodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) {
	byID := make(map[string]*ingestionv1.PipelineNode, len(nodes))
	for _, n := range nodes {
		byID[n.GetId()] = n
	}
	for _, node := range nodes {
		if node.GetKind() != ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK {
			continue
		}
		field := a.sinkSchemaField(ctx, tenant, node)
		if field == "" || node.GetConfig().GetFields()[field].GetStringValue() != "" {
			continue
		}
		schema := naming.Normalize(a.upstreamSourceName(ctx, tenant, node.GetId(), byID, edges))
		if schema == "" {
			continue
		}
		if node.Config == nil {
			node.Config = &structpb.Struct{}
		}
		if node.Config.Fields == nil {
			node.Config.Fields = map[string]*structpb.Value{}
		}
		node.Config.Fields[field] = structpb.NewStringValue(schema)
	}
}

// sinkSchemaField resolves the node's connector and returns its declared
// schema field, or "" when the sink has none or resolution fails.
func (a *Server) sinkSchemaField(ctx context.Context, tenant filament.TenantID, node *ingestionv1.PipelineNode) string {
	conn, err := a.store.LoadConnection(ctx, tenant, node.GetConnectionId())
	if err != nil {
		return ""
	}
	snk, err := a.sinks.Resolve(conn.Connector)
	if err != nil {
		return ""
	}
	return snk.Spec().SchemaField
}

// upstreamSourceName returns the name of the single source connection feeding
// the sink node, or "" when there is none or more than one.
func (a *Server) upstreamSourceName(ctx context.Context, tenant filament.TenantID, sinkID string, nodes map[string]*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) string {
	var connID string
	for _, e := range edges {
		if e.GetToNode() != sinkID {
			continue
		}
		src := nodes[e.GetFromNode()]
		if src == nil || src.GetKind() != ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
			continue
		}
		if connID != "" && connID != src.GetConnectionId() {
			return ""
		}
		connID = src.GetConnectionId()
	}
	if connID == "" {
		return ""
	}
	conn, err := a.store.LoadConnection(ctx, tenant, connID)
	if err != nil {
		return ""
	}
	return conn.Name
}
