package source

import (
	"testing"
	"time"

	rmq "github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

// OpenStream registers NotifyClose after NewConsumer returns. A connection can
// close in between, so the client must retain an event emitted before registration.
func TestConsumerCloseBeforeNotificationRegistration(t *testing.T) {
	coordinator := rmq.NewCoordinator()
	consumer, err := coordinator.NewConsumer(nil, rmq.NewConsumerOptions().SetManualCommit(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.RemoveConsumerById(consumer.GetID(), rmq.Event{Reason: rmq.SocketClosed}); err != nil {
		t.Fatal(err)
	}

	events := consumer.NotifyClose()
	select {
	case event, ok := <-events:
		if !ok || event.Reason != rmq.SocketClosed {
			t.Fatalf("close notification = %+v, open = %v", event, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer closed before registration but no notification was retained")
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("unexpected second close notification")
		}
	case <-time.After(time.Second):
		t.Fatal("close notification channel remained open")
	}
}
