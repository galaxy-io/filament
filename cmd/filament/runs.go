package main

import (
	"context"

	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) listRuns(ctx context.Context, request climodel.RunListRequest) error {
	result, err := a.service.Runs(ctx, request)
	if err != nil {
		return err
	}
	return textrenderer.Runs(a.stdout, result, a.target.Name)
}
