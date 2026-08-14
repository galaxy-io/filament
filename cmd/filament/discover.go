package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

func (a *cliApp) discoverSource(ctx context.Context, args []string, doc configDocument) error {
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
	if len(result.Resources) == 0 {
		_, err := fmt.Fprintf(a.stdout, "No resources discovered for source %q.\n", label)
		return err
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		return result.Resources[i].Name < result.Resources[j].Name
	})

	table := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "Resource\tDisplay name\tSelectable\tPrimary key\tEstimated rows"); err != nil {
		return err
	}
	for _, resource := range result.Resources {
		selectable := "no"
		if resource.Selectable {
			selectable = "yes"
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\n",
			resource.Name, resource.DisplayName, selectable, strings.Join(resource.PrimaryKey, ","), resource.Estimated); err != nil {
			return err
		}
	}
	return table.Flush()
}
