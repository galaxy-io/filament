package slack

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/galaxy-io/filament/internal/notifier"
	"github.com/galaxy-io/filament/internal/notifier/webhook"
)

var _ notifier.Sender = (*Sender)(nil)

// Sender posts notification messages to Slack incoming webhooks.
type Sender struct {
	http *webhook.Sender
}

// New shares the webhook sender's transport rules. A nil client gets the
// guarded transport.
func New(client *http.Client) *Sender {
	return &Sender{http: webhook.New(client)}
}

// Send makes one request. The event bus owns retries.
func (s *Sender) Send(ctx context.Context, n notifier.Notification) (result notifier.DeliveryResult) {
	started := time.Now()
	defer func() { result.Duration = time.Since(started) }()
	result.Outcome = notifier.OutcomeFailed
	if err := ctx.Err(); err != nil {
		result.Retryable = true
		result.ErrorCode = notifier.ErrorTransport
		if errors.Is(err, context.DeadlineExceeded) {
			result.ErrorCode = notifier.ErrorTimeout
		}
		return result
	}
	destination, err := ParseDestination(n.Config)
	if err != nil {
		result.ErrorCode = notifier.ErrorInvalidConfiguration
		return result
	}
	m, err := render(n)
	if err != nil {
		result.ErrorCode = notifier.ErrorInternal
		return result
	}
	body, err := json.Marshal(m)
	if err != nil {
		result.ErrorCode = notifier.ErrorInternal
		return result
	}
	return s.http.Post(ctx, destination.URL, nil, body)
}
