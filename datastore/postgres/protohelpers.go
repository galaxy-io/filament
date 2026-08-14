package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

func cloneProto(p *ingestionv1.Pipeline) *ingestionv1.Pipeline {
	return proto.Clone(p).(*ingestionv1.Pipeline)
}

func marshalConnectionConfig(c filament.Connection) ([]byte, []byte, error) {
	cfg := c.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal connection config: %w", err)
	}
	refs := c.SecretRefs
	if refs == nil {
		refs = map[string]string{}
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal connection secret_refs: %w", err)
	}
	return configJSON, refsJSON, nil
}

func connectionKindToDB(kind filament.ConnectorKind) sqlcgen.ConnectionKind {
	if kind == filament.ConnectorKindSink {
		return sqlcgen.ConnectionKindSink
	}
	return sqlcgen.ConnectionKindSource
}

func connectionKindFromDB(kind sqlcgen.ConnectionKind) filament.ConnectorKind {
	if kind == sqlcgen.ConnectionKindSink {
		return filament.ConnectorKindSink
	}
	if kind == sqlcgen.ConnectionKindSource {
		return filament.ConnectorKindSource
	}
	return filament.ConnectorKindUnspecified
}

func connectionFromRow(id, tenant string, kind sqlcgen.ConnectionKind, name, provider string, configJSON, refsJSON []byte, version int64, createdAt, updatedAt, deletedAt pgtype.Timestamptz, createdBy, updatedBy, deletedBy pgtype.Text) (filament.Connection, error) {
	cfg := map[string]any{}
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return filament.Connection{}, fmt.Errorf("datastore/postgres: unmarshal connection config: %w", err)
		}
	}
	refs := map[string]string{}
	if len(refsJSON) > 0 {
		if err := json.Unmarshal(refsJSON, &refs); err != nil {
			return filament.Connection{}, fmt.Errorf("datastore/postgres: unmarshal connection secret_refs: %w", err)
		}
	}
	return filament.Connection{ID: id, Tenant: tenant, Kind: connectionKindFromDB(kind), Name: name, Connector: provider, Config: cfg, SecretRefs: refs, Version: version, CreatedAt: timestampMillis(createdAt), UpdatedAt: timestampMillis(updatedAt), DeletedAt: timestampMillis(deletedAt), CreatedByUserID: createdBy.String, UpdatedByUserID: updatedBy.String, DeletedByUserID: deletedBy.String}, nil
}
