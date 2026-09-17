package source

import (
	"fmt"
	"maps"

	"github.com/galaxy-io/filament"
)

var _ filament.ReplicationStreamPlanner = (*Source)(nil)

// PlanReplicationStream validates the existing dedicated consumer without
// network access. Provisioning and deleting the consumer remain user-owned.
func (s *Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if err := s.Validate(req.Config); err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	resource, consumer := req.Config.String("stream"), req.Config.String("consumer")
	if resource == "" || consumer == "" {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: stream and consumer are required")
	}
	if len(req.Resources) > 0 && (len(req.Resources) != 1 || req.Resources[0] != resource) {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("nats: select exactly the configured stream")
	}
	return filament.ReplicationStreamPlan{Resources: []string{resource}, ConsumerName: consumer, ConsumerConfig: map[string]any{"stream": resource}, ContinuityConfig: map[string]any{"url": req.Config.String("url"), "source_identity": req.Config.String("source_identity"), "stream": resource, "consumer": consumer}}, nil
}

// BindReplicationStream verifies the admitted consumer rather than provisioning one.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	cfg := filament.NewConfig(config)
	if cfg.String("consumer") != admitted.ConsumerName || cfg.String("stream") != admitted.ConsumerConfig["stream"] {
		return nil, fmt.Errorf("nats: configuration does not match admitted consumer")
	}
	return maps.Clone(config), nil
}
