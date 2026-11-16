package main

import (
	"github.com/galaxy-io/filament/cmd/internal/uiconfig"
	"github.com/galaxy-io/filament/ui"
)

// standaloneConfig is what the standalone process reads from its
// environment before it starts anything.
type standaloneConfig struct {
	webhooks standaloneWebhookConfig
	ui       ui.Config
}

func standaloneConfigFromEnv() (standaloneConfig, error) {
	webhooks, err := standaloneWebhooksFromEnv()
	if err != nil {
		return standaloneConfig{}, err
	}
	uiConfig, err := uiconfig.FromEnv()
	if err != nil {
		return standaloneConfig{}, err
	}
	return standaloneConfig{webhooks: webhooks, ui: uiConfig}, nil
}
