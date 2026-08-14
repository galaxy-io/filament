package compile

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
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
			cfg:  workerConfiguration(filament.WorkerResources{CPURequest: "500m", MemoryLimit: "2Gi"}),
		},
		{
			name: "requests below limits",
			cfg: workerConfiguration(filament.WorkerResources{
				CPURequest: "500m", CPULimit: "1",
				MemoryRequest: "1Gi", MemoryLimit: "2Gi",
			}),
		},
		{
			name: "requests equal limits",
			cfg: workerConfiguration(filament.WorkerResources{
				CPURequest: "1", CPULimit: "1000m",
				MemoryRequest: "1Gi", MemoryLimit: "1024Mi",
			}),
		},
		{
			name: "invalid quantity",
			cfg:  workerConfiguration(filament.WorkerResources{CPURequest: "half a core"}),
			want: ErrInvalid,
		},
		{
			name: "negative quantity",
			cfg:  workerConfiguration(filament.WorkerResources{MemoryLimit: "-1Gi"}),
			want: ErrInvalid,
		},
		{
			name: "cpu request above limit",
			cfg:  workerConfiguration(filament.WorkerResources{CPURequest: "2", CPULimit: "1500m"}),
			want: ErrInvalid,
		},
		{
			name: "memory request above limit",
			cfg:  workerConfiguration(filament.WorkerResources{MemoryRequest: "2Gi", MemoryLimit: "1024Mi"}),
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

func workerConfiguration(resources filament.WorkerResources) filament.WorkerConfiguration {
	return filament.WorkerConfiguration{Resources: resources}
}
