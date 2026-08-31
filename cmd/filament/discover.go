package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/galaxy-io/filament"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) discoverSource(ctx context.Context, args []string, doc climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	allowed := map[string]bool{"refresh": true}
	var connectorName, label string
	var spec filament.ConnectorSpec
	var config map[string]any
	if name == "" {
		connectorName = lastFlag(parsed.flags, "source-connector")
		if connectorName == "" {
			return fmt.Errorf("a saved source name or --source-connector is required; example: filament source discover --source-connector postgres --source-dsn postgres://user:password@host/database")
		}
		var ok bool
		spec, ok = a.catalog.Sources[connectorName]
		if !ok {
			return fmt.Errorf("unknown source connector %q", connectorName)
		}
		allowed["source-connector"] = true
		config, err = directConnectorConfig("source", spec.Config, parsed.flags, allowed)
		if err != nil {
			return err
		}
		label = connectorName
	} else {
		conn, ok := doc.Sources[name]
		if !ok {
			return fmt.Errorf("source %q does not exist", name)
		}
		connectorName = conn.Type
		spec, ok = a.catalog.Sources[connectorName]
		if !ok {
			return fmt.Errorf("source %q: unknown connector %q", name, connectorName)
		}
		if err := validateConnectionFields("source "+name, spec.Config, conn); err != nil {
			return err
		}
		config, err = resolvedConnectionConfig(conn, nil, spec.Config)
		if err != nil {
			return fmt.Errorf("source %q: %w", name, err)
		}
		config, err = overlayScopedFlags(config, "source-", spec.Config, parsed.flags, allowed)
		if err != nil {
			return err
		}
		label = name
	}
	if err := rejectUnknownFlags(parsed.flags, allowed); err != nil {
		return err
	}
	refresh := false
	if raw, present := flagValue(parsed.flags, "refresh"); present {
		refresh, err = strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("--refresh must be true or false")
		}
	}

	resources, err := a.service.Discover(ctx, climodel.DiscoverRequest{
		Connector: connectorName,
		Source:    label,
		Config:    config,
		Refresh:   refresh,
	})
	if err != nil {
		return err
	}
	return textrenderer.Resources(a.stdout, resources)
}
