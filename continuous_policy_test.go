package filament_test

import (
	"testing"

	"github.com/galaxy-io/filament"
	kafka "github.com/galaxy-io/filament/connectors/kafka/sink"
	nats "github.com/galaxy-io/filament/connectors/nats/sink"
	postgres "github.com/galaxy-io/filament/connectors/postgres/sink"
)

type auditPolicySource struct {
	filament.Source
	spec filament.ConnectorSpec
}

func (s auditPolicySource) Spec() filament.ConnectorSpec { return s.spec }
func TestSavedContinuousSinkPoliciesRemainAdmissible(t *testing.T) {
	rows := filament.ConnectorSpec{Stream: &filament.StreamCapabilities{Input: filament.InputRows, Delivery: filament.DeliveryReplayableAtLeastOnce}}
	changes := filament.ConnectorSpec{Stream: &filament.StreamCapabilities{Input: filament.InputChanges, Delivery: filament.DeliveryReplayableAtLeastOnce, EmitsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}}}
	oldMessage := filament.WritePolicyCapability{Mode: filament.WriteAppend, AcceptsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Atomicity: filament.AtomicityRecord, Durability: filament.DurabilityAfterApply}
	oldPostgres := filament.WriteCapabilities(filament.IngestionFullAppend)[0]
	oldPostgres.Durability = filament.DurabilityAfterCommit
	for _, tc := range []struct {
		name   string
		sink   filament.Sink
		source filament.ConnectorSpec
		policy filament.WritePolicyCapability
	}{
		{"kafka", kafka.New(), changes, oldMessage}, {"nats", nats.New(), changes, oldMessage}, {"postgres", postgres.New(), rows, oldPostgres},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := filament.RunSpec{Resources: []string{"items"}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: tc.policy}}}
			policies, _, err := filament.PlanContinuousRun(auditPolicySource{spec: tc.source}, tc.sink, spec)
			if err != nil {
				t.Fatal(err)
			}
			if policies["items"].Capability.RequiresPK || policies["items"].Capability.RequiresOrder {
				t.Fatal("legacy keyless/unordered append restricted")
			}
			plan, err := filament.PlanContinuousWrite(tc.source, tc.sink.Spec(), filament.WriteAppend)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Policy.Capability.Durability != tc.policy.Durability {
				t.Fatalf("durability changed: %+v", plan.Policy.Capability)
			}
		})
	}
}
