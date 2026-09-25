package source

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func TestConsumerContract(t *testing.T) {
	c := jetstream.ConsumerConfig{Durable: "consumer", AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if err := validateExistingConsumer(c, "consumer"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*jetstream.ConsumerConfig){func(c *jetstream.ConsumerConfig) { c.MaxAckPending = 10 }, func(c *jetstream.ConsumerConfig) { c.AckPolicy = jetstream.AckAllPolicy }, func(c *jetstream.ConsumerConfig) { c.FilterSubject = "x" }, func(c *jetstream.ConsumerConfig) { c.MaxDeliver = 1 }, func(c *jetstream.ConsumerConfig) { c.DeliverPolicy = jetstream.DeliverNewPolicy }} {
		bad := c
		mutate(&bad)
		if validateExistingConsumer(bad, "consumer") == nil {
			t.Fatal("unsafe consumer accepted")
		}
	}
}

func TestConsumerBindingValidation(t *testing.T) {
	orders := consumerBinding{stream: "EVENTS", consumer: "orders", resource: "orders.>", filters: []string{"orders.>"}, managed: true}
	products := consumerBinding{stream: "EVENTS", consumer: "products", resource: "products.>", filters: []string{"products.>"}, managed: true}
	config := jetstream.ConsumerConfig{Durable: "orders", Description: "Filament managed orders", FilterSubject: "orders.>", AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if err := orders.validateConsumer(config); err != nil {
		t.Fatal(err)
	}
	if err := products.validateConsumer(config); err == nil {
		t.Fatal("another resource's consumer accepted")
	}
	for _, mutate := range []func(*jetstream.ConsumerConfig){
		func(c *jetstream.ConsumerConfig) { c.FilterSubject = "products.>" },
		func(c *jetstream.ConsumerConfig) { c.Description = "user-owned" },
		func(c *jetstream.ConsumerConfig) { c.MaxAckPending = 2 },
	} {
		bad := config
		mutate(&bad)
		if orders.validateConsumer(bad) == nil {
			t.Fatal("incompatible managed consumer accepted")
		}
	}
	config.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
	config.OptStartSeq = 42
	if err := orders.validateConsumer(config); err != nil {
		t.Fatalf("managed resume rejected: %v", err)
	}
	existing := consumerBinding{stream: "EVENTS", consumer: "orders", resource: "EVENTS"}
	if existing.validateConsumer(config) == nil {
		t.Fatal("explicit consumer accepted managed delivery contract")
	}
}

func TestConsumerProgress(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		committed, initial, stream, consumer uint64
		wantErr                              bool
	}{
		{name: "new consumer on retained input", initial: 4, stream: 4},
		{name: "older server starts at zero", initial: 4},
		{name: "uncertified acknowledgement", initial: 4, stream: 5, consumer: 1, wantErr: true},
		{name: "retention cannot hide acknowledgement", initial: 9, stream: 5, consumer: 1, wantErr: true},
		{name: "starting floor cannot advance", initial: 4, stream: 6, wantErr: true},
		{name: "explicit consumer has no bootstrap floor", stream: 4, wantErr: true},
		{name: "certified acknowledgement", committed: 5, initial: 4, stream: 5, consumer: 1},
		{name: "resume cannot bootstrap", committed: 3, initial: 4, stream: 4, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &jetstream.ConsumerInfo{AckFloor: jetstream.SequenceInfo{Stream: tc.stream, Consumer: tc.consumer}}
			if err := validateConsumerProgress(info, tc.committed, tc.initial); (err != nil) != tc.wantErr {
				t.Fatalf("validateConsumerProgress: %v, want error=%v", err, tc.wantErr)
			}
		})
	}
}
