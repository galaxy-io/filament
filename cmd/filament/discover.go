package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/galaxy-io/filament"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textoutput "github.com/galaxy-io/filament/cmd/internal/cli/output/text"
	localtarget "github.com/galaxy-io/filament/cmd/internal/cli/target/local"
	"github.com/galaxy-io/filament/registry"
)

func (a *cliApp) discoverSource(ctx context.Context, args []string, doc localtarget.Document) error {
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
		spec, ok = a.catalog.sources[connectorName]
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
		spec, ok = a.catalog.sources[connectorName]
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

	source, err := registry.DefaultSources.Resolve(connectorName)
	if err != nil {
		return fmt.Errorf("resolve source connector %q: %w", connectorName, err)
	}
	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return fmt.Errorf("source connector %q does not support resource discovery", connectorName)
	}
	if err := source.Configure(ctx, filament.NewConfig(config)); err != nil {
		return fmt.Errorf("configure source %q: %w", label, err)
	}
	defer func() { _ = source.Teardown(ctx) }()

	result, err := discoverable.Discover(ctx, filament.DiscoverOpts{Refresh: refresh})
	if err != nil {
		return fmt.Errorf("discover source %q: %w", label, err)
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		return result.Resources[i].Name < result.Resources[j].Name
	})
	resources := climodel.ResourceList{Source: label, Items: make([]climodel.ResourceSummary, 0, len(result.Resources))}
	for _, resource := range result.Resources {
		resources.Items = append(resources.Items, climodel.ResourceSummary{
			Name:          resource.Name,
			DisplayName:   resource.DisplayName,
			Selectable:    resource.Selectable,
			PrimaryKey:    append([]string(nil), resource.PrimaryKey...),
			EstimatedRows: resource.Estimated,
		})
	}
	return textoutput.Resources(a.stdout, resources)
}
