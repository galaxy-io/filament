package k8sdispatch

import (
	"context"
	"errors"
	"time"

	ingestion "github.com/galaxy-io/filament"
)

type runHandle struct {
	run ingestion.RunID
	ds  ingestion.DataStore
}

func (h runHandle) ID() ingestion.RunID { return h.run }

func (h runHandle) Status(ctx context.Context) (ingestion.RunStatus, error) {
	if h.ds == nil {
		return 0, errors.New("k8sdispatch: handle has no datastore")
	}
	state, err := h.ds.LoadRun(ctx, h.run)
	if err != nil {
		return 0, err
	}
	return state.Status, nil
}

func (h runHandle) Wait(ctx context.Context) (ingestion.RunResult, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if h.ds == nil {
			return ingestion.RunResult{}, errors.New("k8sdispatch: handle has no datastore")
		}
		state, err := h.ds.LoadRun(ctx, h.run)
		if err != nil {
			return ingestion.RunResult{}, err
		}
		switch state.Status {
		case ingestion.RunCompleted, ingestion.RunFailed, ingestion.RunCanceled:
			return ingestion.RunResult{
				Status:  state.Status,
				Records: state.Records,
				Bytes:   state.Bytes,
				Error:   state.Error,
			}, nil
		}
		select {
		case <-ctx.Done():
			return ingestion.RunResult{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
