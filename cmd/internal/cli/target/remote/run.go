package remote

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/events"
)

// SubmitRun asks the deployment to run a saved pipeline. Inline specs and
// override flags are work only the local runner executes.
func (t *Target) SubmitRun(ctx context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	if submission.Pipeline == nil {
		return model.RunGroup{}, errors.New("this target runs saved pipelines; create the pipeline first")
	}
	if submission.Override {
		return model.RunGroup{}, errors.New("this target does not take override flags; edit the pipeline instead")
	}
	id := submission.Pipeline.Metadata.ID
	if id == "" {
		existing, err := t.findPipeline(ctx, submission.Pipeline.Name)
		if err != nil {
			return model.RunGroup{}, err
		}
		id = existing.GetId()
	}
	response, err := t.client.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{
		PipelineId: id, ClientToken: uuid.NewString(),
	}))
	if err != nil {
		return model.RunGroup{}, t.rpcError(err)
	}
	edgeRuns := response.Msg.GetEdgeRuns()
	if len(edgeRuns) == 0 {
		return model.RunGroup{}, fmt.Errorf("pipeline %q produced no runs", submission.Pipeline.Name)
	}
	group := model.RunGroup{Runs: make([]model.RunRef, 0, len(edgeRuns))}
	for _, edgeRun := range edgeRuns {
		group.Runs = append(group.Runs, model.RunRef{ID: edgeRun.GetRun().GetId(), Route: edgeRun.GetPipelineEdgeKey()})
	}
	return group, nil
}

// TailRun follows every run in the group over the deployment's event stream
// and reports normalized resource progress. The stream closes only after the
// deployment has persisted the terminal status, so no waiting happens here.
func (t *Target) TailRun(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	var mu sync.Mutex
	locked := func(event model.RunEvent) {
		if observe == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		observe(event)
	}
	results := make([]tailResult, len(group.Runs))
	errs := make([]error, len(group.Runs))
	var tails sync.WaitGroup
	for i, ref := range group.Runs {
		tails.Add(1)
		go func() {
			defer tails.Done()
			results[i], errs[i] = t.tailOne(ctx, ref, locked)
		}()
	}
	tails.Wait()
	total := model.RunResult{Runs: group.Runs, Status: "complete"}
	for i, result := range results {
		total.Records += result.records
		total.Bytes += result.bytes
		if statusRank(result.status) > statusRank(total.Status) {
			total.Status = result.status
		}
		if errs[i] != nil && result.status == "" {
			total.Status = "failed"
		}
	}
	return total, errors.Join(errs...)
}

// statusRank orders terminal statuses so a group reports its worst member.
func statusRank(status string) int {
	switch status {
	case "failed":
		return 5
	case "partial":
		return 4
	case "canceled":
		return 3
	case "paused":
		return 2
	case "complete":
		return 1
	default:
		return 0
	}
}

type tailResult struct {
	status  string
	records int64
	bytes   int64
}

func (t *Target) tailOne(ctx context.Context, ref model.RunRef, observe func(model.RunEvent)) (tailResult, error) {
	stream, err := t.client.TailRun(ctx, connect.NewRequest(&ingestionv1.TailRunRequest{RunId: ref.ID, ShouldReplay: true}))
	if err != nil {
		return tailResult{}, t.rpcError(err)
	}
	defer func() { _ = stream.Close() }()
	for stream.Receive() {
		event := stream.Msg().GetEvent()
		fields := event.GetFields()
		switch event.GetEventType() {
		case events.ResourceStarted.Name():
			observe(runEvent(ref, event.GetResource(), "running"))
		case events.BatchWritten.Name():
			update := runEvent(ref, event.GetResource(), "running")
			update.Records, update.Bytes = fields.GetRecords(), fields.GetBytes()
			observe(update)
		case events.ResourceCompleted.Name():
			update := runEvent(ref, event.GetResource(), "complete")
			update.Records, update.Bytes, update.Final = fields.GetRecords(), fields.GetBytes(), true
			observe(update)
		case events.ResourceFailed.Name():
			update := runEvent(ref, event.GetResource(), "failed")
			update.Error = fields.GetError()
			observe(update)
		case events.RunCompleted.Name():
			return tailResult{status: "complete", records: fields.GetRecords(), bytes: fields.GetBytes()}, nil
		case events.RunFailed.Name():
			return tailResult{status: "failed"}, fmt.Errorf("run %s failed: %s", ref.ID, fields.GetError())
		case events.RunPartial.Name():
			return tailResult{status: "partial"}, fmt.Errorf("run %s is partial: %s", ref.ID, fields.GetError())
		case events.RunPaused.Name():
			observe(model.RunEvent{Run: ref.ID, Route: ref.Route, Status: "paused", Final: true})
			return tailResult{status: "paused"}, nil
		case events.RunCanceled.Name():
			observe(model.RunEvent{Run: ref.ID, Route: ref.Route, Status: "canceled", Final: true})
			return tailResult{status: "canceled"}, nil
		}
	}
	if err := stream.Err(); err != nil {
		return tailResult{}, t.rpcError(err)
	}
	return tailResult{}, errors.New("run event stream closed before completion")
}

// SignalRun sends a lifecycle signal to a deployment run.
func (t *Target) SignalRun(ctx context.Context, ref model.RunRef, signal filament.Signal) error {
	protoSignal, err := signalToProto(signal)
	if err != nil {
		return err
	}
	_, err = t.client.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{RunId: ref.ID, Signal: protoSignal}))
	return t.rpcError(err)
}

func signalToProto(signal filament.Signal) (ingestionv1.RunSignal, error) {
	switch signal {
	case filament.SignalPause:
		return ingestionv1.RunSignal_RUN_SIGNAL_PAUSE, nil
	case filament.SignalResume:
		return ingestionv1.RunSignal_RUN_SIGNAL_RESUME, nil
	case filament.SignalCancel:
		return ingestionv1.RunSignal_RUN_SIGNAL_CANCEL, nil
	default:
		return ingestionv1.RunSignal_RUN_SIGNAL_UNSPECIFIED, fmt.Errorf("unknown run signal %d", signal)
	}
}

func runEvent(ref model.RunRef, resource, status string) model.RunEvent {
	return model.RunEvent{Run: ref.ID, Route: ref.Route, Resource: resource, Status: status}
}
