package compile

import (
	"fmt"
	"maps"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// WorkerConfigurationFromProto converts the wire form to the domain form. A nil
// message is the zero configuration, which inherits everything.
func WorkerConfigurationFromProto(cfg *ingestionv1.WorkerConfiguration) filament.WorkerConfiguration {
	out := filament.WorkerConfiguration{
		Resources: filament.WorkerResources{
			Requests: cloneNonEmpty(cfg.GetResources().GetRequests()),
			Limits:   cloneNonEmpty(cfg.GetResources().GetLimits()),
		},
		NodeSelector: cloneNonEmpty(cfg.GetNodeSelector()),
	}
	for _, t := range cfg.GetTolerations() {
		out.Tolerations = append(out.Tolerations, filament.WorkerToleration{
			Key:      t.GetKey(),
			Operator: t.GetOperator(),
			Value:    t.GetValue(),
			Effect:   t.GetEffect(),
		})
	}
	return out
}

func cloneNonEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return maps.Clone(m)
}

// ValidateWorkerConfiguration rejects values Kubernetes would refuse, so a bad
// value fails at the API write where a human sees it rather than at Job
// creation. Empty fields are unset and always valid.
func ValidateWorkerConfiguration(cfg filament.WorkerConfiguration) error {
	if err := validateWorkerResources(cfg.Resources); err != nil {
		return err
	}
	for key, value := range cfg.NodeSelector {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return fmt.Errorf("%w: worker nodeSelector key %q: %s", ErrInvalid, key, strings.Join(errs, "; "))
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("%w: worker nodeSelector value %q: %s", ErrInvalid, value, strings.Join(errs, "; "))
		}
	}
	for i, t := range cfg.Tolerations {
		if err := validateWorkerToleration(t); err != nil {
			return fmt.Errorf("%w: worker tolerations[%d]: %w", ErrInvalid, i, err)
		}
	}
	return nil
}

// validateWorkerResources checks that every quantity parses, is non-negative,
// and that no request exceeds its limit.
func validateWorkerResources(res filament.WorkerResources) error {
	parse := func(kind string, in map[string]string) (map[string]resource.Quantity, error) {
		out := make(map[string]resource.Quantity, len(in))
		for name, value := range in {
			if errs := validation.IsQualifiedName(name); len(errs) > 0 {
				return nil, fmt.Errorf("%w: worker resources.%s name %q: %s", ErrInvalid, kind, name, strings.Join(errs, "; "))
			}
			quantity, err := resource.ParseQuantity(value)
			if err != nil {
				return nil, fmt.Errorf("%w: worker resources.%s.%s %q: %w", ErrInvalid, kind, name, value, err)
			}
			if quantity.Sign() < 0 {
				return nil, fmt.Errorf("%w: worker resources.%s.%s %q must be non-negative", ErrInvalid, kind, name, value)
			}
			out[name] = quantity
		}
		return out, nil
	}
	requests, err := parse("requests", res.Requests)
	if err != nil {
		return err
	}
	limits, err := parse("limits", res.Limits)
	if err != nil {
		return err
	}
	for name, request := range requests {
		if limit, ok := limits[name]; ok && request.Cmp(limit) > 0 {
			return fmt.Errorf("%w: worker resources.requests.%s must be less than or equal to resources.limits.%s", ErrInvalid, name, name)
		}
	}
	return nil
}

// validateWorkerToleration applies the rules the API server enforces on a
// toleration.
func validateWorkerToleration(t filament.WorkerToleration) error {
	switch t.Operator {
	case "", "Equal":
		if t.Key == "" {
			return fmt.Errorf("key is required unless operator is Exists")
		}
	case "Exists":
		if t.Value != "" {
			return fmt.Errorf("value must be empty when operator is Exists")
		}
	default:
		return fmt.Errorf("operator %q must be Equal or Exists", t.Operator)
	}
	if t.Key != "" {
		if errs := validation.IsQualifiedName(t.Key); len(errs) > 0 {
			return fmt.Errorf("key %q: %s", t.Key, strings.Join(errs, "; "))
		}
	}
	if t.Value != "" {
		if errs := validation.IsValidLabelValue(t.Value); len(errs) > 0 {
			return fmt.Errorf("value %q: %s", t.Value, strings.Join(errs, "; "))
		}
	}
	switch t.Effect {
	case "", "NoSchedule", "PreferNoSchedule", "NoExecute":
		return nil
	default:
		return fmt.Errorf("effect %q must be NoSchedule, PreferNoSchedule, or NoExecute", t.Effect)
	}
}
