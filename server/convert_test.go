package server

import (
	"slices"
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/registry"

	_ "github.com/galaxy-io/filament/connectors/mysql"
	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

func supportedTypesFor(t *testing.T, kind ingestionv1.ConnectorKind, name string) []ingestionv1.IngestionType {
	t.Helper()
	if kind == ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
		for _, spec := range registry.DefaultSources.Specs() {
			if spec.Name == name {
				return sourceSpecToProto(spec).GetSupportedIngestionTypes()
			}
		}
	} else {
		for _, spec := range registry.DefaultSinks.Specs() {
			if spec.Name == name {
				return sinkSpecToProto(spec).GetSupportedIngestionTypes()
			}
		}
	}
	t.Fatalf("connector %q not registered", name)
	return nil
}

func TestConnectorSpecSupportedIngestionTypes(t *testing.T) {
	tests := []struct {
		kind ingestionv1.ConnectorKind
		name string
		want []ingestionv1.IngestionType
	}{
		{
			kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
			name: "postgres",
			want: []ingestionv1.IngestionType{
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT,
				ingestionv1.IngestionType_INGESTION_TYPE_APPEND,
			},
		},
		{
			kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
			name: "mysql",
			want: []ingestionv1.IngestionType{
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT,
				ingestionv1.IngestionType_INGESTION_TYPE_APPEND,
				ingestionv1.IngestionType_INGESTION_TYPE_CDC,
			},
		},
		{
			kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
			name: "stdout",
			want: []ingestionv1.IngestionType{
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
				ingestionv1.IngestionType_INGESTION_TYPE_APPEND,
			},
		},
		{
			kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
			name: "postgres",
			want: []ingestionv1.IngestionType{
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT,
				ingestionv1.IngestionType_INGESTION_TYPE_APPEND,
				ingestionv1.IngestionType_INGESTION_TYPE_UPSERT,
			},
		},
		{
			kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
			name: "mysql",
			want: []ingestionv1.IngestionType{
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
				ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT,
				ingestionv1.IngestionType_INGESTION_TYPE_APPEND,
				ingestionv1.IngestionType_INGESTION_TYPE_UPSERT,
				ingestionv1.IngestionType_INGESTION_TYPE_CDC,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := supportedTypesFor(t, tt.kind, tt.name)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
