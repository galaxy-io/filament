package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/galaxy-io/filament/internal/notifier"
)

var _ notifier.Sender = (*Sender)(nil)

// Sender posts notification events to webhook destinations.
type Sender struct {
	client *http.Client
}

// New reuses the client's transport without changing the caller's client. A nil
// client gets the standard transport with a dialer that refuses private and
// local addresses after resolution, which also closes DNS rebinding.
func New(client *http.Client) *Sender {
	if client == nil {
		client = &http.Client{Transport: guardedTransport()}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Sender{client: &clientCopy}
}

// Send makes one request. The event bus owns retries.
func (s *Sender) Send(ctx context.Context, n notifier.Notification) (result notifier.DeliveryResult) {
	started := time.Now()
	defer func() { result.Duration = time.Since(started) }()
	result.Outcome = notifier.OutcomeFailed
	if ctx.Err() != nil {
		result.Retryable = true
		result.ErrorCode = transportErrorCode(ctx.Err())
		return result
	}
	destination, err := ParseDestination(n.Config)
	if err != nil {
		result.ErrorCode = notifier.ErrorInvalidConfiguration
		return result
	}
	body, err := json.Marshal(payload{
		SchemaVersion: "1", NotifierID: n.NotifierID, DeliveryID: n.DeliveryID, PipelineID: n.PipelineID, PipelineVersionID: n.PipelineVersionID,
		TriggerStreamSequence: n.TriggerStreamSequence, Event: n.Event,
	})
	if err != nil {
		result.Retryable = true
		result.ErrorCode = notifier.ErrorInternal
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destination.URL, bytes.NewReader(body))
	if err != nil {
		result.Retryable = true
		result.ErrorCode = notifier.ErrorInvalidConfiguration
		return result
	}
	for name, value := range destination.Headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Filament-Delivery-ID", n.DeliveryID)
	req.Header.Set("X-Filament-Attempt-ID", n.AttemptID)
	req.Header.Set("X-Filament-Event-Type", n.TriggerType)
	result.RequestAttempted = true
	response, err := s.client.Do(req)
	if errors.Is(err, errPrivateAddress) {
		result.ErrorCode = notifier.ErrorInvalidConfiguration
		return result
	}
	if err != nil {
		result.Outcome = transportOutcome(err)
		result.Retryable = true
		result.ErrorCode = transportErrorCode(err)
		return result
	}
	defer func() { _ = response.Body.Close() }()
	// Drain a small response for connection reuse. Never retain its contents.
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4*1024))
	result.StatusCode = response.StatusCode
	result.Outcome, result.Retryable, result.ErrorCode = statusResult(response.StatusCode)
	if result.Retryable {
		result.RetryAfter = retryAfter(response.Header.Get("Retry-After"))
	}
	return result
}

// guardedTransport dials only public addresses, checking every resolved IP.
func guardedTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		err = errPrivateAddress
		for _, ip := range ips {
			if !publicAddr(ip) {
				continue
			}
			var conn net.Conn
			if conn, err = dialer.DialContext(ctx, network, net.JoinHostPort(ip.Unmap().String(), port)); err == nil {
				return conn, nil
			}
		}
		return nil, err
	}
	return t
}

// retryAfter reads a Retry-After header as seconds or an HTTP date. Zero when absent or unreadable.
func retryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		if d := time.Until(at); d > 0 {
			return d
		}
	}
	return 0
}

type payload struct {
	SchemaVersion         string          `json:"schema_version"`
	NotifierID            string          `json:"notifier_id"`
	DeliveryID            string          `json:"delivery_id"`
	PipelineID            string          `json:"pipeline_id"`
	PipelineVersionID     string          `json:"pipeline_version_id"`
	TriggerStreamSequence uint64          `json:"trigger_stream_sequence,string"`
	Event                 json.RawMessage `json:"event"`
}

func statusResult(status int) (notifier.Outcome, bool, notifier.ErrorCode) {
	if status >= 200 && status < 300 {
		return notifier.OutcomeAccepted, false, notifier.ErrorNone
	}
	if status == http.StatusTooManyRequests {
		return notifier.OutcomeFailed, true, notifier.ErrorRateLimited
	}
	retryable := status == http.StatusRequestTimeout || status == http.StatusTooEarly || status >= 500 && status < 600
	return notifier.OutcomeFailed, retryable, notifier.ErrorHTTPRejected
}

func transportOutcome(err error) notifier.Outcome {
	var dnsError *net.DNSError
	var opError *net.OpError
	if errors.As(err, &dnsError) || errors.As(err, &opError) && opError.Op == "dial" {
		return notifier.OutcomeFailed
	}
	return notifier.OutcomeUnknown
}

func transportErrorCode(err error) notifier.ErrorCode {
	var netError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netError) && netError.Timeout() {
		return notifier.ErrorTimeout
	}
	return notifier.ErrorTransport
}
