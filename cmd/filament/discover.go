package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) discoverSource(ctx context.Context, args []string, document climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	request := cliapp.DiscoverSourceRequest{Source: firstPositional(parsed)}
	allowed := map[string]bool{"refresh": true}
	if request.Source == "" {
		request.Connector = lastFlag(parsed.flags, "source-connector")
		if request.Connector == "" {
			return fmt.Errorf("a saved source name or --source-connector is required; example: filament source discover --source-connector postgres --source-dsn postgres://user:password@host/database")
		}
		allowed["source-connector"] = true
		schema, schemaErr := a.catalog.ConnectionSchema("source", request.Connector)
		if schemaErr != nil {
			return schemaErr
		}
		request.Config, _, err = configPatchFromAllFlags(schema, "source-", parsed.flags, allowed, true)
	} else {
		connection, ok := document.Sources[request.Source]
		if !ok {
			return fmt.Errorf("source %q does not exist", request.Source)
		}
		schema, schemaErr := a.catalog.ConnectionSchema("source", connection.Type)
		if schemaErr != nil {
			return schemaErr
		}
		request.Config, _, err = configPatchFromFlags(schema, filament.ScopePipeline, "source-", parsed.flags, allowed, false)
	}
	if err != nil {
		return err
	}
	if err := rejectUnknownFlags(parsed.flags, allowed); err != nil {
		return err
	}
	if raw, present := flagValue(parsed.flags, "refresh"); present {
		request.Refresh, err = strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("--refresh must be true or false")
		}
	}
	resources, err := a.service.DiscoverSource(ctx, request)
	if err != nil {
		return err
	}
	return textrenderer.Resources(a.stdout, resources)
}
