package source

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/galaxy-io/filament"
)

type streamBinding struct {
	Stream   string `json:"stream"`
	Consumer string `json:"consumer"`
}

// configuredStreams validates the explicit stream/consumer bindings.
func configuredStreams(cfg filament.Config) ([]streamBinding, error) {
	if cfg.Has("stream") || cfg.Has("consumer") {
		return nil, fmt.Errorf("nats: explicit consumers must be configured in streams")
	}
	var bindings []streamBinding
	raw, err := json.Marshal(cfg.Raw()["streams"])
	if err != nil {
		return nil, fmt.Errorf("nats: streams: %w", err)
	}
	if err := json.Unmarshal(raw, &bindings); err != nil {
		return nil, fmt.Errorf("nats: streams must be an array of stream/consumer objects: %w", err)
	}
	if len(bindings) == 0 {
		return nil, fmt.Errorf("nats: configure at least one stream and consumer")
	}
	slices.SortFunc(bindings, func(a, b streamBinding) int {
		if a.Stream < b.Stream {
			return -1
		}
		if a.Stream > b.Stream {
			return 1
		}
		return 0
	})
	for i, b := range bindings {
		if b.Stream == "" || b.Consumer == "" {
			return nil, fmt.Errorf("nats: each stream requires a stream and consumer name")
		}
		if i > 0 && bindings[i-1].Stream == b.Stream {
			return nil, fmt.Errorf("nats: duplicate stream %q", b.Stream)
		}
	}
	return bindings, nil
}

func selectStreams(bindings []streamBinding, resources []string) ([]streamBinding, error) {
	if len(resources) == 0 {
		return slices.Clone(bindings), nil
	}
	byName := make(map[string]streamBinding, len(bindings))
	for _, b := range bindings {
		byName[b.Stream] = b
	}
	out := make([]streamBinding, 0, len(resources))
	seen := map[string]bool{}
	for _, resource := range resources {
		b, ok := byName[resource]
		if !ok || seen[resource] {
			return nil, fmt.Errorf("nats: resource %q is unconfigured or duplicated", resource)
		}
		seen[resource] = true
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b streamBinding) int {
		if a.Stream < b.Stream {
			return -1
		}
		if a.Stream > b.Stream {
			return 1
		}
		return 0
	})
	return out, nil
}
