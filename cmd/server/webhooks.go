package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/health"
	"github.com/galaxy-io/filament/server"
	"github.com/galaxy-io/filament/telemetry"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

// webhookReceiver is the receiver, its endpoint cache, the inbox it fills,
// the address it listens on, and the public origin senders reach it on.
type webhookReceiver struct {
	receiver      *webhooks.Receiver
	cache         *webhooks.EndpointCache
	inbox         *inbox.Inbox
	addr          string
	publicBaseURL string
}

// apiOption lets endpoint configuration respect the inbox size and refresh
// this replica's cache. Other replicas pick a change up within the cache TTL.
func (w *webhookReceiver) apiOption() server.Option {
	return server.WithWebhookReceiver(server.WebhookReceiver{
		MaxBody: w.inbox.MaxBody(), Invalidate: w.cache.Invalidate, PublicBaseURL: w.publicBaseURL,
	})
}

// listen serves the delivery routes on their own listener, so a public
// webhook ingress never reaches the API, and keeps stored endpoints in step
// with their source connectors. Health is served too for load balancer
// checks. A listener error goes to errCh.
func (w *webhookReceiver) listen(ctx context.Context, api *server.Server, healthState *health.State, errCh chan<- error) *http.Server {
	api.StartWebhookReconciler(ctx)
	mux := http.NewServeMux()
	healthState.Mount(mux)
	w.receiver.Mount(mux)
	srv := &http.Server{
		Addr: w.addr, Handler: otelhttp.NewHandler(mux, "webhooks"),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() { errCh <- srv.ListenAndServe() }()
	return srv
}

// webhooksFromEnv provisions the durable inbox only when explicitly
// enabled. Deliveries are authenticated by their signature, not by Connect
// user authentication.
func webhooksFromEnv(ctx context.Context, store any, secrets filament.Secrets, metrics *telemetry.Set) (*webhookReceiver, func(), error) {
	enabled, err := parseWebhookEnabled(os.Getenv("WEBHOOKS_ENABLED"))
	if err != nil || !enabled {
		return nil, nil, err
	}
	loader, ok := store.(webhooks.EndpointLoader)
	if !ok || secrets == nil {
		return nil, nil, errors.New("webhooks require an endpoint store and secrets provider")
	}
	cfg, err := webhookInboxConfigFromEnv()
	if err != nil {
		return nil, nil, err
	}
	maxInFlight := 64
	if raw := os.Getenv("WEBHOOK_MAX_IN_FLIGHT"); raw != "" {
		maxInFlight, err = strconv.Atoi(raw)
		if err != nil || maxInFlight < 1 || maxInFlight > 10000 {
			return nil, nil, errors.New("WEBHOOK_MAX_IN_FLIGHT must be between 1 and 10000")
		}
	}
	addr := os.Getenv("WEBHOOK_ADDR")
	if addr == "" {
		addr = ":8088"
	}
	publicBaseURL, err := parsePublicBaseURL(os.Getenv("WEBHOOK_PUBLIC_BASE_URL"))
	if err != nil {
		return nil, nil, err
	}
	url := os.Getenv("NATS_URL")
	if url == "" {
		return nil, nil, errors.New("webhooks require NATS_URL")
	}
	conn, err := nats.Connect(url, nats.Name("filament-webhook-receiver"), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, nil, fmt.Errorf("webhooks: connect NATS: %w", err)
	}
	// A message the broker cannot carry would fail every delivery near the
	// endpoint limit, so refuse to start instead.
	if int64(cfg.MaxMsgSize) > conn.MaxPayload() {
		conn.Close()
		return nil, nil, fmt.Errorf("WEBHOOK_INBOX_MAX_MESSAGE_BYTES %d exceeds the NATS server max_payload %d",
			cfg.MaxMsgSize, conn.MaxPayload())
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("webhooks: JetStream: %w", err)
	}
	i, err := inbox.Open(ctx, js, cfg)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	cache := &webhooks.EndpointCache{Loader: loader, Secrets: secrets}
	receiver := &webhooks.Receiver{
		Endpoints: cache, Inbox: i, Metrics: metrics,
		Slots: make(chan struct{}, maxInFlight),
	}
	return &webhookReceiver{receiver: receiver, cache: cache, inbox: i, addr: addr, publicBaseURL: publicBaseURL}, conn.Close, nil
}

// parsePublicBaseURL accepts an absolute http or https origin, optionally
// with a path prefix, and returns it without a trailing slash.
func parsePublicBaseURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := neturl.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("WEBHOOK_PUBLIC_BASE_URL must be an absolute http or https URL without query or fragment: %q", raw)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func parseWebhookEnabled(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("WEBHOOKS_ENABLED must be a boolean: %w", err)
	}
	return enabled, nil
}

func webhookInboxConfigFromEnv() (inbox.Config, error) {
	read := func(name string) (int64, error) {
		raw := os.Getenv(name)
		if raw == "" {
			return 0, fmt.Errorf("%s is required when webhooks are enabled", name)
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", name)
		}
		return value, nil
	}
	maxBytes, err := read("WEBHOOK_INBOX_MAX_BYTES")
	if err != nil {
		return inbox.Config{}, err
	}
	messageBytes, err := read("WEBHOOK_INBOX_MAX_MESSAGE_BYTES")
	if err != nil {
		return inbox.Config{}, err
	}
	replicas, err := read("WEBHOOK_INBOX_REPLICAS")
	if err != nil {
		return inbox.Config{}, err
	}
	if messageBytes > maxBytes || messageBytes > 1<<31-1 || messageBytes <= inbox.HeaderAllowance || replicas > 5 {
		return inbox.Config{}, errors.New("webhook inbox capacity, message size, or replica count is invalid")
	}
	return inbox.Config{MaxBytes: maxBytes, MaxMsgSize: int32(messageBytes), Replicas: int(replicas)}, nil
}
