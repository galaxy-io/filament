package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	webhooksource "github.com/galaxy-io/filament/connectors/webhook"
	"github.com/galaxy-io/filament/internal/modules/webhookretention"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

const standaloneNATSMaxPayload = 32 << 20

type standaloneWebhookConfig struct {
	enabled       bool
	inbox         inbox.Config
	maxInFlight   int
	publicBaseURL string
}

func standaloneWebhooksFromEnv() (standaloneWebhookConfig, error) {
	var config standaloneWebhookConfig
	if raw := os.Getenv("WEBHOOKS_ENABLED"); raw != "" {
		var err error
		config.enabled, err = strconv.ParseBool(raw)
		if err != nil {
			return config, fmt.Errorf("WEBHOOKS_ENABLED must be a boolean: %w", err)
		}
	}
	if !config.enabled {
		return config, nil
	}
	read := func(name string, fallback int64) (int64, error) {
		raw := os.Getenv(name)
		if raw == "" {
			return fallback, nil
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", name)
		}
		return value, nil
	}
	maxBytes, err := read("WEBHOOK_INBOX_MAX_BYTES", 64<<20)
	if err != nil {
		return config, err
	}
	messageBytes, err := read("WEBHOOK_INBOX_MAX_MESSAGE_BYTES", 2<<20)
	if err != nil {
		return config, err
	}
	replicas, err := read("WEBHOOK_INBOX_REPLICAS", 1)
	if err != nil {
		return config, err
	}
	maxInFlight, err := read("WEBHOOK_MAX_IN_FLIGHT", 64)
	if err != nil {
		return config, err
	}
	if messageBytes > maxBytes || messageBytes > standaloneNATSMaxPayload ||
		messageBytes <= inbox.HeaderAllowance || replicas != 1 || maxInFlight > 10000 {
		return config, errors.New("standalone webhook inbox capacity, message size, replica count, or concurrency is invalid")
	}
	config.inbox = inbox.Config{MaxBytes: maxBytes, MaxMsgSize: int32(messageBytes), Replicas: 1}
	config.maxInFlight = int(maxInFlight)
	if raw := os.Getenv("WEBHOOK_PUBLIC_BASE_URL"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
			return config, errors.New("WEBHOOK_PUBLIC_BASE_URL must be an absolute http or https URL without query or fragment")
		}
		config.publicBaseURL = strings.TrimRight(u.String(), "/")
	}
	return config, nil
}

func openStandaloneWebhooks(ctx context.Context, conn *nats.Conn, config standaloneWebhookConfig) (*inbox.Inbox, *webhookretention.Module, error) {
	if !config.enabled {
		return nil, nil, nil
	}
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, nil, err
	}
	i, err := inbox.Open(ctx, js, config.inbox)
	if err != nil {
		return nil, nil, err
	}
	retention, err := webhookretention.NewFromEnv(func(context.Context) (*inbox.Inbox, func(), error) {
		return i, func() {}, nil
	})
	return i, retention, err
}

// standaloneSources swaps the webhook source's NATS_URL attacher for the
// in-process inbox used by the standalone receiver.
type standaloneSources struct {
	filament.SourceRegistry
	inbox *inbox.Inbox
}

func (s standaloneSources) Resolve(name string) (filament.Source, error) {
	if name == "webhook" && s.inbox != nil {
		if _, err := s.SourceRegistry.Resolve(name); err != nil {
			return nil, err
		}
		return webhooksource.NewWithInbox(s.inbox), nil
	}
	return s.SourceRegistry.Resolve(name)
}
