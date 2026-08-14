package compile

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// WorkerConfigurationFromProto converts the wire form to the domain form. A nil
// message is the zero configuration, which inherits everything.
func WorkerConfigurationFromProto(cfg *ingestionv1.WorkerConfiguration) filament.WorkerConfiguration {
	return filament.WorkerConfiguration{
		Resources: filament.WorkerResources{
			CPURequest:    cfg.GetResources().GetCpuRequest(),
			CPULimit:      cfg.GetResources().GetCpuLimit(),
			MemoryRequest: cfg.GetResources().GetMemoryRequest(),
			MemoryLimit:   cfg.GetResources().GetMemoryLimit(),
		},
	}
}

// ValidateWorkerConfiguration rejects quantities and request/limit pairs that
// Kubernetes would refuse, so a bad value fails at the API write where a human
// sees it rather than at Job creation. Empty fields are unset and always valid.
func ValidateWorkerConfiguration(cfg filament.WorkerConfiguration) error {
	quantities := make(map[string]resource.Quantity, 4)
	for _, q := range []struct {
		field string
		value string
	}{
		{"cpu_request", cfg.Resources.CPURequest},
		{"cpu_limit", cfg.Resources.CPULimit},
		{"memory_request", cfg.Resources.MemoryRequest},
		{"memory_limit", cfg.Resources.MemoryLimit},
	} {
		if q.value == "" {
			continue
		}
		quantity, err := resource.ParseQuantity(q.value)
		if err != nil {
			return fmt.Errorf("%w: worker %s %q: %w", ErrInvalid, q.field, q.value, err)
		}
		if quantity.Sign() < 0 {
			return fmt.Errorf("%w: worker %s %q must be non-negative", ErrInvalid, q.field, q.value)
		}
		quantities[q.field] = quantity
	}
	for _, pair := range []struct {
		request string
		limit   string
	}{
		{"cpu_request", "cpu_limit"},
		{"memory_request", "memory_limit"},
	} {
		request, hasRequest := quantities[pair.request]
		limit, hasLimit := quantities[pair.limit]
		if hasRequest && hasLimit && request.Cmp(limit) > 0 {
			return fmt.Errorf("%w: worker %s must be less than or equal to %s", ErrInvalid, pair.request, pair.limit)
		}
	}
	return nil
}
