package source

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestConsumerContract(t *testing.T) {
	c := nats.ConsumerConfig{Durable: "consumer", AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if err := validateConsumer(c, "consumer"); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*nats.ConsumerConfig){func(c *nats.ConsumerConfig) { c.MaxAckPending = 10 }, func(c *nats.ConsumerConfig) { c.AckPolicy = nats.AckAllPolicy }, func(c *nats.ConsumerConfig) { c.FilterSubject = "x" }, func(c *nats.ConsumerConfig) { c.MaxDeliver = 1 }, func(c *nats.ConsumerConfig) { c.DeliverPolicy = nats.DeliverNewPolicy }} {
		bad := c
		mutate(&bad)
		if validateConsumer(bad, "consumer") == nil {
			t.Fatal("unsafe consumer accepted")
		}
	}
}
