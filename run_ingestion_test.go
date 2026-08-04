package filament

import (
	"slices"
	"testing"
)

func TestSupportedSourceIngestionTypes(t *testing.T) {
	tests := []struct {
		name string
		spec ConnectorSpec
		want []IngestionType
	}{
		{
			name: "full only",
			spec: ConnectorSpec{
				Modes:          []ReplicationMode{ModeFull},
				SourcePolicies: []SourcePolicy{SourcePolicyForIngestion(IngestionSnapshotReplace)},
			},
			want: []IngestionType{IngestionSnapshotReplace, IngestionSnapshotUpsert, IngestionAppend},
		},
		{
			name: "full and cdc",
			spec: ConnectorSpec{
				Modes: []ReplicationMode{ModeFull, ModeCDC},
				SourcePolicies: []SourcePolicy{
					SourcePolicyForIngestion(IngestionSnapshotReplace),
					SourcePolicyForIngestion(IngestionCDC),
				},
			},
			want: []IngestionType{
				IngestionSnapshotReplace, IngestionSnapshotUpsert, IngestionAppend, IngestionCDC,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SupportedSourceIngestionTypes(tt.spec)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSupportedSinkIngestionTypes(t *testing.T) {
	tests := []struct {
		name string
		spec SinkSpec
		want []IngestionType
	}{
		{
			name: "append and replace only",
			spec: SinkSpec{
				Capabilities: SinkCapabilities{
					WritePolicies: []WritePolicyCapability{
						WritePolicyForIngestion(IngestionSnapshotReplace).Capability,
						WritePolicyForIngestion(IngestionAppend).Capability,
					},
				},
			},
			want: []IngestionType{IngestionSnapshotReplace, IngestionAppend},
		},
		{
			name: "upsertable fallback without declared upsert policy",
			spec: SinkSpec{
				Capabilities: SinkCapabilities{Upsertable: true},
			},
			want: []IngestionType{
				IngestionSnapshotReplace, IngestionSnapshotUpsert, IngestionAppend, IngestionUpsert,
			},
		},
		{
			name: "merge capable",
			spec: SinkSpec{
				Capabilities: SinkCapabilities{
					Upsertable: true,
					WritePolicies: []WritePolicyCapability{
						WritePolicyForIngestion(IngestionSnapshotReplace).Capability,
						WritePolicyForIngestion(IngestionAppend).Capability,
						WritePolicyForIngestion(IngestionSnapshotUpsert).Capability,
						WritePolicyForIngestion(IngestionUpsert).Capability,
						WritePolicyForIngestion(IngestionCDC).Capability,
					},
				},
			},
			want: []IngestionType{
				IngestionSnapshotReplace, IngestionSnapshotUpsert, IngestionAppend,
				IngestionUpsert, IngestionCDC,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SupportedSinkIngestionTypes(tt.spec)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
