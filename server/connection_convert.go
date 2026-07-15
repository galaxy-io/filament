package server

import (
	"google.golang.org/protobuf/types/known/structpb"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func connectionFromProto(c *ingestionv1.Connection) ingestion.Connection {
	if c == nil {
		return ingestion.Connection{}
	}
	return ingestion.Connection{ID: c.GetId(), Tenant: c.GetTenant(), Kind: connectionKindFromProto(c.GetKind()), Name: c.GetName(), Connector: c.GetConnector(), Config: structMap(c.GetConfig()), SecretRefs: cloneStrings(c.GetSecretRefs()), Version: c.GetVersion()}
}

func connectionToProto(c ingestion.Connection) *ingestionv1.Connection {
	cfg, _ := structpb.NewStruct(c.Config)
	return &ingestionv1.Connection{Id: c.ID, Tenant: c.Tenant, Kind: connectionKindToProto(c.Kind), Name: c.Name, Connector: c.Connector, Config: cfg, SecretRefs: cloneStrings(c.SecretRefs), Version: c.Version}
}

func connectionKindFromProto(k ingestionv1.ConnectorKind) ingestion.ConnectorKind {
	switch k {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		return ingestion.ConnectorKindSource
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		return ingestion.ConnectorKindSink
	default:
		return ingestion.ConnectorKindUnspecified
	}
}

func connectionKindToProto(k ingestion.ConnectorKind) ingestionv1.ConnectorKind {
	switch k {
	case ingestion.ConnectorKindSource:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE
	case ingestion.ConnectorKindSink:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK
	default:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED
	}
}
