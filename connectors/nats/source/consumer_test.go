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
