package server

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func connectionFromProto(c *ingestionv1.Connection) filament.Connection {
	if c == nil {
		return filament.Connection{}
	}
	return filament.Connection{ID: c.GetId(), Tenant: c.GetTenantId(), Kind: connectionKindFromProto(c.GetKind()), Name: c.GetName(), Connector: c.GetConnector(), Config: structMap(c.GetConfig()), SecretRefs: cloneStrings(c.GetSecretRefs()), Version: c.GetVersion(), CreatedAt: c.GetCreatedAt(), UpdatedAt: c.GetUpdatedAt(), DeletedAt: c.GetDeletedAt(), CreatedByUserID: c.GetCreatedByUserId(), UpdatedByUserID: c.GetUpdatedByUserId(), DeletedByUserID: c.GetDeletedByUserId()}
}

func connectionToProto(c filament.Connection) *ingestionv1.Connection {
	cfg, _ := structpb.NewStruct(c.Config)
	return &ingestionv1.Connection{Id: c.ID, TenantId: c.Tenant, Kind: connectionKindToProto(c.Kind), Name: c.Name, Connector: c.Connector, Config: cfg, SecretRefs: cloneStrings(c.SecretRefs), Version: c.Version, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: c.DeletedAt, CreatedByUserId: c.CreatedByUserID, UpdatedByUserId: c.UpdatedByUserID, DeletedByUserId: c.DeletedByUserID}
}

func connectionKindFromProto(k ingestionv1.ConnectorKind) filament.ConnectorKind {
	switch k {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		return filament.ConnectorKindSource
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		return filament.ConnectorKindSink
	default:
		return filament.ConnectorKindUnspecified
	}
}

func connectionKindToProto(k filament.ConnectorKind) ingestionv1.ConnectorKind {
	switch k {
	case filament.ConnectorKindSource:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE
	case filament.ConnectorKindSink:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK
	default:
		return ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED
	}
}
