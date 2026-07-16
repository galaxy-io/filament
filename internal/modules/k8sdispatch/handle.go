package k8sdispatch

import (
	"context"
	"errors"
	"time"

	"github.com/galaxy-io/filament"
)

type runHandle struct {
	run filament.RunID
	ds  filament.DataStore
}

func (h runHandle) ID() filament.RunID { return h.run }

func (h runHandle) Status(ctx context.Context) (filament.RunStatus, error) {
	if h.ds == nil {
		return 0, errors.New("k8sdispatch: handle has no datastore")
	}
	state, err := h.ds.LoadRun(ctx, h.run)
	if err != nil {
		return 0, err
	}
	return state.Status, nil
}

func (h runHandle) Wait(ctx context.Context) (filament.RunResult, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if h.ds == nil {
			return filament.RunResult{}, errors.New("k8sdispatch: handle has no datastore")
		}
		state, err := h.ds.LoadRun(ctx, h.run)
		if err != nil {
			return filament.RunResult{}, err
		}
		switch state.Status {
		case filament.RunCompleted, filament.RunFailed, filament.RunCanceled:
			return filament.RunResult{
				Status:  state.Status,
				Records: state.Records,
				Bytes:   state.Bytes,
				Error:   state.Error,
			}, nil
		}
		select {
		case <-ctx.Done():
			return filament.RunResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
