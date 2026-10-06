package source

import (
	"fmt"
	"maps"
	"slices"

	"github.com/galaxy-io/filament"
)

var _ filament.ReplicationStreamPlanner = (*Source)(nil)

// PlanReplicationStream fixes continuous membership to configured/selected RabbitMQ Streams.
func (s *Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if err := s.Validate(req.Config); err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	streams, err := selectedStreams(req.Resources, configStrings(req.Config, "streams"))
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	if req.SourceConnectionID == "" {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("rabbitmq: source connection ID is required")
	}
	return filament.ReplicationStreamPlan{
		Resources:        streams,
		ConsumerName:     "rabbitmq-streams",
		ConsumerConfig:   map[string]any{"streams": slices.Clone(streams)},
		ContinuityConfig: map[string]any{"source_connection_id": req.SourceConnectionID, "streams": slices.Clone(streams)},
	}, nil
}

// BindReplicationStream verifies that the stored configuration still matches admitted stream membership.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	bound, err := selectedStreams(nil, configStrings(filament.NewConfig(admitted.ConsumerConfig), "streams"))
	if err != nil {
		return nil, err
	}
	configured, err := selectedStreams(nil, configStrings(filament.NewConfig(config), "streams"))
	if err != nil || !slices.Equal(configured, bound) {
		return nil, fmt.Errorf("rabbitmq: configuration does not match admitted streams")
	}
	return maps.Clone(config), nil
}
