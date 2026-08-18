package compile

import (
	"errors"
	"reflect"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestValidateWorkerConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  filament.WorkerConfiguration
		want error
	}{
		{name: "empty"},
		{
			name: "partial",
			cfg:  resources(map[string]string{"cpu": "500m"}, map[string]string{"memory": "2Gi"}),
		},
		{
			name: "requests below limits",
			cfg:  resources(map[string]string{"cpu": "500m", "memory": "1Gi"}, map[string]string{"cpu": "1", "memory": "2Gi"}),
		},
		{
			name: "requests equal limits",
			cfg:  resources(map[string]string{"cpu": "1", "memory": "1Gi"}, map[string]string{"cpu": "1000m", "memory": "1024Mi"}),
		},
		{
			name: "extended resource",
			cfg:  resources(nil, map[string]string{"nvidia.com/gpu": "1"}),
		},
		{
			name: "invalid quantity",
			cfg:  resources(map[string]string{"cpu": "half a core"}, nil),
			want: ErrInvalid,
		},
		{
			name: "negative quantity",
			cfg:  resources(nil, map[string]string{"memory": "-1Gi"}),
			want: ErrInvalid,
		},
		{
			name: "bad resource name",
			cfg:  resources(map[string]string{"not a resource": "1"}, nil),
			want: ErrInvalid,
		},
		{
			name: "cpu request above limit",
			cfg:  resources(map[string]string{"cpu": "2"}, map[string]string{"cpu": "1500m"}),
			want: ErrInvalid,
		},
		{
			name: "memory request above limit",
			cfg:  resources(map[string]string{"memory": "2Gi"}, map[string]string{"memory": "1024Mi"}),
			want: ErrInvalid,
		},
		{
			name: "node selector",
			cfg: filament.WorkerConfiguration{
				NodeSelector: map[string]string{"eks.amazonaws.com/nodegroup": "private", "tier": "workers"},
			},
		},
		{
			name: "tolerations",
			cfg: filament.WorkerConfiguration{Tolerations: []filament.WorkerToleration{
				{Key: "dedicated", Operator: "Equal", Value: "filament", Effect: "NoSchedule"},
				{Key: "spot", Operator: "Exists"},
				{Operator: "Exists"},
			}},
		},
		{
			name: "bad selector key",
			cfg:  filament.WorkerConfiguration{NodeSelector: map[string]string{"not a key": "x"}},
			want: ErrInvalid,
		},
		{
			name: "bad selector value",
			cfg:  filament.WorkerConfiguration{NodeSelector: map[string]string{"tier": "has spaces"}},
			want: ErrInvalid,
		},
		{
			name: "equal without key",
			cfg:  filament.WorkerConfiguration{Tolerations: []filament.WorkerToleration{{Value: "x"}}},
			want: ErrInvalid,
		},
		{
			name: "exists with value",
			cfg:  filament.WorkerConfiguration{Tolerations: []filament.WorkerToleration{{Key: "k", Operator: "Exists", Value: "x"}}},
			want: ErrInvalid,
		},
		{
			name: "unknown operator",
			cfg:  filament.WorkerConfiguration{Tolerations: []filament.WorkerToleration{{Key: "k", Operator: "Like"}}},
			want: ErrInvalid,
		},
		{
			name: "unknown effect",
			cfg:  filament.WorkerConfiguration{Tolerations: []filament.WorkerToleration{{Key: "k", Effect: "Never"}}},
			want: ErrInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkerConfiguration(tt.cfg)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateWorkerConfiguration() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func resources(requests, limits map[string]string) filament.WorkerConfiguration {
	return filament.WorkerConfiguration{Resources: filament.WorkerResources{Requests: requests, Limits: limits}}
}

func TestWorkerConfigurationFromProto(t *testing.T) {
	got := WorkerConfigurationFromProto(&ingestionv1.WorkerConfiguration{
		Resources:    &ingestionv1.WorkerResources{Requests: map[string]string{"cpu": "500m"}, Limits: map[string]string{"memory": "2Gi"}},
		NodeSelector: map[string]string{"tier": "workers"},
		Tolerations:  []*ingestionv1.WorkerToleration{{Key: "spot", Operator: "Exists", Effect: "NoSchedule"}},
	})
	want := filament.WorkerConfiguration{
		Resources:    filament.WorkerResources{Requests: map[string]string{"cpu": "500m"}, Limits: map[string]string{"memory": "2Gi"}},
		NodeSelector: map[string]string{"tier": "workers"},
		Tolerations:  []filament.WorkerToleration{{Key: "spot", Operator: "Exists", Effect: "NoSchedule"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !WorkerConfigurationFromProto(nil).IsZero() {
		t.Fatal("nil proto must be the zero configuration")
	}
}
