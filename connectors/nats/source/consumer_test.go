package source

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestConsumerContract(t *testing.T) {
	c := nats.ConsumerConfig{Durable: "consumer", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if err := validateExistingConsumer(c, "consumer"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nats.ConsumerConfig){func(c *nats.ConsumerConfig) { c.MaxAckPending = 10 }, func(c *nats.ConsumerConfig) { c.AckPolicy = nats.AckAllPolicy }, func(c *nats.ConsumerConfig) { c.FilterSubject = "x" }, func(c *nats.ConsumerConfig) { c.MaxDeliver = 1 }, func(c *nats.ConsumerConfig) { c.DeliverPolicy = nats.DeliverNewPolicy }} {
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
	config := nats.ConsumerConfig{Durable: "orders", Description: "Filament managed orders", FilterSubject: "orders.>", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if err := orders.validateConsumer(config); err != nil {
		t.Fatal(err)
	}
	if err := products.validateConsumer(config); err == nil {
		t.Fatal("another resource's consumer accepted")
	}
	for _, mutate := range []func(*nats.ConsumerConfig){
		func(c *nats.ConsumerConfig) { c.FilterSubject = "products.>" },
		func(c *nats.ConsumerConfig) { c.Description = "user-owned" },
		func(c *nats.ConsumerConfig) { c.MaxAckPending = 2 },
	} {
		bad := config
		mutate(&bad)
		if orders.validateConsumer(bad) == nil {
			t.Fatal("incompatible managed consumer accepted")
		}
	}
	config.DeliverPolicy = nats.DeliverByStartSequencePolicy
	config.OptStartSeq = 42
	if err := orders.validateConsumer(config); err != nil {
		t.Fatalf("managed resume rejected: %v", err)
	}
	existing := consumerBinding{stream: "EVENTS", consumer: "orders", resource: "EVENTS"}
	if existing.validateConsumer(config) == nil {
		t.Fatal("explicit consumer accepted managed delivery contract")
	}
}
