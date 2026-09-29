package slack

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

// slackClient routes every request to the test server while keeping the
// Slack hostname the destination validates.
func slackClient(server *httptest.Server) *http.Client {
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	return &http.Client{Transport: transport}
}

func notification() notifier.Notification {
	return notifier.Notification{
		PipelineName: "orders", TriggerType: "run.failed",
		TriggerEvent: ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_FAILED,
		Event:        json.RawMessage(`{"type":"run.failed","run_id":"run-1","at":"2026-09-29T14:02:00Z"}`),
		Config:       json.RawMessage(`{"url":"https://hooks.slack.com/services/T000/B000/XXXX"}`),
	}
}

func TestSendPostsBlocks(t *testing.T) {
	var path, contentType string
	var body message
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, contentType = r.URL.Path, r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	result := New(slackClient(server)).Send(context.Background(), notification())
	if result.Outcome != notifier.OutcomeAccepted || result.Retryable || result.StatusCode != http.StatusOK {
		t.Fatalf("Send() = %+v", result)
	}
	if path != "/services/T000/B000/XXXX" || contentType != "application/json" {
		t.Fatalf("request path %q, content type %q", path, contentType)
	}
	if len(body.Attachments) != 1 || body.Attachments[0].Color != red || body.Attachments[0].Fallback != "Filament run failed · orders" {
		t.Fatalf("body = %+v", body)
	}
}

func TestSendClassifiesResponses(t *testing.T) {
	for _, tc := range []struct {
		status     int
		retryAfter string
		retryable  bool
		code       notifier.ErrorCode
		wait       time.Duration
	}{
		{status: http.StatusBadRequest, code: notifier.ErrorHTTPRejected},
		{status: http.StatusNotFound, code: notifier.ErrorHTTPRejected},
		{status: http.StatusGone, code: notifier.ErrorHTTPRejected},
		{status: http.StatusTooManyRequests, retryAfter: "7", retryable: true, code: notifier.ErrorRateLimited, wait: 7 * time.Second},
		{status: http.StatusInternalServerError, retryable: true, code: notifier.ErrorHTTPRejected},
	} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if tc.retryAfter != "" {
				w.Header().Set("Retry-After", tc.retryAfter)
			}
			w.WriteHeader(tc.status)
		}))
		result := New(slackClient(server)).Send(context.Background(), notification())
		server.Close()
		if result.Outcome != notifier.OutcomeFailed || result.Retryable != tc.retryable || result.ErrorCode != tc.code || result.RetryAfter != tc.wait {
			t.Errorf("status %d: Send() = %+v", tc.status, result)
		}
	}
}

func TestSendRejectsInvalidDestination(t *testing.T) {
	n := notification()
	n.Config = json.RawMessage(`{"url":"https://example.com/services/T000/B000/XXXX"}`)
	result := New(nil).Send(context.Background(), n)
	if result.RequestAttempted || result.Retryable || result.ErrorCode != notifier.ErrorInvalidConfiguration {
		t.Fatalf("Send() = %+v", result)
	}
}
