package server

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/convert"
)

func connectionFromProto(c *ingestionv1.Connection) filament.Connection {
	if c == nil {
		return filament.Connection{}
	}
	return filament.Connection{ID: c.GetId(), Tenant: c.GetTenantId(), Kind: convert.ConnectorKindFromProto(c.GetKind()), Name: c.GetName(), Connector: c.GetConnector(), Config: convert.StructMap(c.GetConfig()), SecretRefs: cloneStrings(c.GetSecretRefs()), Version: c.GetVersion(), CreatedAt: c.GetCreatedAt(), UpdatedAt: c.GetUpdatedAt(), DeletedAt: c.GetDeletedAt(), CreatedByUserID: c.GetCreatedByUserId(), UpdatedByUserID: c.GetUpdatedByUserId(), DeletedByUserID: c.GetDeletedByUserId()}
}

func connectionToProto(c filament.Connection) *ingestionv1.Connection {
	cfg, _ := structpb.NewStruct(c.Config)
	return &ingestionv1.Connection{Id: c.ID, TenantId: c.Tenant, Kind: convert.ConnectorKindToProto(c.Kind), Name: c.Name, Connector: c.Connector, Config: cfg, SecretRefs: cloneStrings(c.SecretRefs), Version: c.Version, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: c.DeletedAt, CreatedByUserId: c.CreatedByUserID, UpdatedByUserId: c.UpdatedByUserID, DeletedByUserId: c.DeletedByUserID}
}
