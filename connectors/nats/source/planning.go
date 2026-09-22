package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/galaxy-io/filament"
)

var _ filament.ReplicationStreamPlanner = (*Source)(nil)

// PlanReplicationStream resolves fixed resource membership without network access.
// The connection identity namespaces progress independently of consumer attempts.
func (s *Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if err := s.Validate(req.Config); err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	identity, err := sourceIdentity(req.SourceConnectionID)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	if !req.Config.Has("streams") && !req.Config.Has("stream") && !req.Config.Has("consumer") {
		subjects := slices.Clone(req.Resources)
		if len(subjects) == 0 && req.Config.Has("subjects") {
			raw, err := json.Marshal(req.Config.Raw()["subjects"])
			if err != nil {
				return filament.ReplicationStreamPlan{}, err
			}
			if err := json.Unmarshal(raw, &subjects); err != nil {
				return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: subjects must be an array of patterns")
			}
		}
		if len(subjects) == 0 {
			return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: select one or more subject patterns, for example orders.>")
		}
		slices.Sort(subjects)
		for i, subject := range subjects {
			if err := validateSubject(subject); err != nil {
				return filament.ReplicationStreamPlan{}, err
			}
			if i > 0 && subjects[i-1] == subject {
				return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: duplicate subject resource %q", subject)
			}
		}
		// Each admitted generation needs its own catalog identity. Keep this out
		// of ContinuityConfig so compatible runs still reuse their admission.
		if req.ReplicationStreamID == "" {
			return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: replication stream ID is required for managed consumers")
		}
		return filament.ReplicationStreamPlan{Resources: subjects, ConsumerName: "filament-" + req.ReplicationStreamID, ConsumerConfig: map[string]any{"subjects": subjects}, ContinuityConfig: map[string]any{"source_identity": identity, "subjects": subjects}}, nil
	}
	if req.Config.Has("subjects") {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: subject resources cannot be combined with explicit stream consumers")
	}
	bindings, err := configuredStreams(req.Config)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	selected, err := selectStreams(bindings, req.Resources)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	resources := make([]string, len(selected))
	for i, b := range selected {
		resources[i] = b.Stream
	}
	name, config := consumerPlan(selected)
	return filament.ReplicationStreamPlan{Resources: resources, ConsumerName: name, ConsumerConfig: config, ContinuityConfig: map[string]any{"url": req.Config.String("url"), "source_identity": identity, "streams": selected}}, nil
}

// The admitted consumer name identifies the logical set, not an extra provider
// consumer. Preserve legacy single-consumer metadata for existing admissions.
func consumerPlan(bindings []streamBinding) (string, map[string]any) {
	if len(bindings) == 1 {
		return bindings[0].Consumer, map[string]any{"stream": bindings[0].Stream}
	}
	raw, _ := json.Marshal(bindings)
	sum := sha256.Sum256(raw)
	return "nats-" + hex.EncodeToString(sum[:]), map[string]any{"streams": bindings}
}

// BindReplicationStream validates and binds source configuration to the admitted consumer identity.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	if subjects, ok := admitted.ConsumerConfig["subjects"]; ok {
		if admitted.ID == "" || admitted.ConsumerName != "filament-"+admitted.ID {
			return nil, fmt.Errorf("nats: managed consumer identity mismatch")
		}
		var patterns []string
		raw, err := json.Marshal(subjects)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &patterns); err != nil || len(patterns) == 0 {
			return nil, fmt.Errorf("nats: invalid admitted subjects")
		}
		cfg := filament.NewConfig(config)
		if cfg.Has("streams") || cfg.Has("stream") || cfg.Has("consumer") {
			return nil, fmt.Errorf("nats: admitted subjects cannot bind explicit consumers")
		}
		for _, pattern := range patterns {
			if err := validateSubject(pattern); err != nil {
				return nil, err
			}
		}
		return maps.Clone(config), nil
	}
	configured, err := configuredStreams(filament.NewConfig(config))
	if err != nil {
		return nil, err
	}
	var bound []streamBinding
	if resource, ok := admitted.ConsumerConfig["stream"].(string); ok {
		bound = []streamBinding{{Stream: resource, Consumer: admitted.ConsumerName}}
	} else {
		bound, err = configuredStreams(filament.NewConfig(admitted.ConsumerConfig))
		if err != nil {
			return nil, err
		}
	}
	resources := make([]string, len(bound))
	for i, b := range bound {
		resources[i] = b.Stream
	}
	selected, err := selectStreams(configured, resources)
	if err != nil || !slices.Equal(selected, bound) {
		return nil, fmt.Errorf("nats: configuration does not match admitted consumers")
	}
	name, _ := consumerPlan(bound)
	if name != admitted.ConsumerName {
		return nil, fmt.Errorf("nats: admitted consumer identity mismatch")
	}
	return maps.Clone(config), nil
}
