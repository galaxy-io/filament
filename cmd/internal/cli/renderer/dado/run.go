package dado

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/atterpac/dado/inline"
	"golang.org/x/term"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

type resourceProgressUpdate struct {
	resource string
	status   string
	records  int64
	bytes    int64
	final    bool
	err      string
}

type runProgressDone struct {
	result model.RunResult
	err    error
}

func (r *Renderer) interactiveRun(ctx context.Context) error {
	listed, err := r.service.Pipelines(ctx)
	if err != nil {
		return err
	}
	if len(listed.Items) == 0 {
		return r.showInteractiveMessage(ctx, "Run a pipeline", "No saved pipelines. Create a source, sink, and pipeline first.")
	}
	options := pipelineMenuOptions(listed.Items)
	options = append(options, interactiveOption{label: "Back", value: interactiveBack})
	name, err := r.chooseInteractive(ctx, "Run a pipeline", "Choose a saved pipeline", options)
	if err != nil || name == interactiveBack {
		return err
	}
	return r.runInteractivePipeline(ctx, name)
}

func (r *Renderer) runInteractivePipeline(ctx context.Context, name string) error {
	return r.runRequest(ctx, cliapp.RunRequest{Pipeline: name})
}

// RunRequest renders one run with the live grid. Interruption ends the run quietly;
// the grid has already reported it.
func (r *Renderer) RunRequest(ctx context.Context, request cliapp.RunRequest) (runErr error) {
	renderer := inline.NewRenderer(inline.WithOutput(r.statusWriter()))
	r.interactiveRenderer = renderer
	defer func() {
		runErr = errors.Join(runErr, renderer.Clear(), renderer.Close())
		r.interactiveRenderer = nil
	}()
	if err := r.runRequest(ctx, request); err != nil && !interactiveInterrupted(err) {
		return err
	}
	return nil
}

func (r *Renderer) runRequest(ctx context.Context, request cliapp.RunRequest) error {
	submission, err := r.service.PrepareRun(ctx, request)
	if err != nil {
		return err
	}
	spec := submission.Spec
	if err := r.announceRun(request.Pipeline, spec); err != nil {
		return err
	}
	var resources []model.ResourceSummary
	if request.Pipeline != "" {
		doc, err := r.service.Configuration(ctx)
		if err != nil {
			return err
		}
		var discoverErr error
		resources, discoverErr = r.discoverPipelineResources(ctx, doc.Pipelines[request.Pipeline])
		if err := r.announceDiscovery(discoverErr); err != nil {
			return err
		}
	}
	if spec.Sink.Connector == "stdout" {
		if output, ok := r.stdout.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
			confirmed, confirmErr := r.confirmInteractive(ctx, "Run with the stdout sink?", "Records stream to stdout while progress renders on stderr. Redirect stdout for a clean display.")
			if confirmErr != nil || !confirmed {
				return confirmErr
			}
		}
	}
	_, err = r.renderInteractiveRun(ctx, submission, resources)
	return err
}

func (r *Renderer) announceRun(name string, spec filament.RunSpec) error {
	route := spec.Source.Connector + " → " + spec.Sink.Connector
	if name != "" {
		route = name + " · " + route
	}
	return r.interactiveRenderer.Println(r.painter().Title("Running", route))
}

func (r *Renderer) announceDiscovery(discoverErr error) error {
	p := r.painter()
	status := p.Success("Ok")
	if discoverErr != nil {
		status = p.Muted("Unavailable")
	}
	return r.interactiveRenderer.Println(style.Indent + p.Label("Discovering Resources… ") + status)
}

func (r *Renderer) renderInteractiveRun(ctx context.Context, submission model.RunSubmission, discovered []model.ResourceSummary) (model.RunResult, error) {
	if r.interactiveRenderer == nil {
		return model.RunResult{}, errors.New("interactive renderer is not running")
	}
	spec := submission.Spec
	group, err := r.service.SubmitRun(ctx, submission)
	if err != nil {
		return model.RunResult{}, err
	}
	view := &runProgressView{theme: r.theme}
	rows := map[string]*runRow{}
	addResource := func(name string) {
		if name == "" || rows[name] != nil {
			return
		}
		row := &runRow{name: name}
		rows[name] = row
		view.rows = append(view.rows, row)
	}
	selected := make(map[string]bool, len(spec.Resources))
	for _, name := range spec.Resources {
		selected[name] = true
	}
	for _, resource := range discovered {
		if len(selected) == 0 || selected[resource.Name] {
			addResource(resource.Name)
		}
	}
	for _, name := range spec.Resources {
		addResource(name)
	}

	updates := make(chan resourceProgressUpdate, 256)
	done := make(chan runProgressDone, 1)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		result, runErr := r.service.TailRun(runCtx, group, func(event model.RunEvent) {
			update := resourceProgressUpdate{
				resource: event.Resource, status: event.Status, records: event.Records,
				bytes: event.Bytes, final: event.Final, err: event.Error,
			}
			select {
			case updates <- update:
			case <-runCtx.Done():
			}
		})
		done <- runProgressDone{result: result, err: runErr}
	}()

	startedAt := time.Now()
	width := interactiveRenderWidth(r.statusWriter())
	render := func() error { return r.interactiveRenderer.Render(view.Frame(width)) }
	if err := render(); err != nil {
		return model.RunResult{}, err
	}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case update := <-updates:
			addResource(update.resource)
			applyRunUpdate(rows[update.resource], update)
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case completed := <-done:
			drainRunUpdates(updates, rows, addResource)
			settleRows(view, completed.err)
			return completed.result, errors.Join(completed.err, r.finishRun(view, completed.result, time.Since(startedAt), completed.err))
		case <-ticker.C:
			view.tick++
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case <-ctx.Done():
			settleRows(view, ctx.Err())
			return model.RunResult{}, errors.Join(ctx.Err(), r.finishRun(view, model.RunResult{}, time.Since(startedAt), ctx.Err()))
		}
	}
}

func applyRunUpdate(row *runRow, update resourceProgressUpdate) {
	if row == nil {
		return
	}
	if update.final {
		row.records = update.records
		row.bytes = update.bytes
	} else {
		row.records += update.records
		row.bytes += update.bytes
	}
	switch update.status {
	case "failed":
		row.state = rowFailed
		row.err = update.err
	case "complete":
		row.state = rowDone
	default:
		row.state = rowRunning
	}
}

// drainRunUpdates applies updates that arrived alongside completion.
func drainRunUpdates(updates <-chan resourceProgressUpdate, rows map[string]*runRow, addResource func(string)) {
	for {
		select {
		case update := <-updates:
			addResource(update.resource)
			applyRunUpdate(rows[update.resource], update)
		default:
			return
		}
	}
}

// settleRows resolves rows still in flight once the run has ended.
func settleRows(view *runProgressView, runErr error) {
	for _, row := range view.rows {
		if row.state != rowPending && row.state != rowRunning {
			continue
		}
		if runErr == nil {
			row.state = rowDone
		} else {
			row.state = rowCancelled
		}
	}
}

// finishRun moves the final rows and the summary into scrollback.
func (r *Renderer) finishRun(view *runProgressView, result model.RunResult, elapsed time.Duration, runErr error) error {
	if err := r.interactiveRenderer.Clear(); err != nil {
		return err
	}
	p := r.painter()
	if err := r.interactiveRenderer.Println("\n" + strings.TrimRight(view.lines(p), "\n")); err != nil {
		return err
	}
	return r.interactiveRenderer.Println("\n" + runSummaryLine(p, len(view.rows), result, elapsed, runErr))
}

func interactiveRenderWidth(writer io.Writer) int {
	if output, ok := writer.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
		if width, _, err := term.GetSize(int(output.Fd())); err == nil && width > 1 {
			return width - 1
		}
	}
	return 96
}

func humanDuration(value time.Duration) string {
	switch {
	case value < time.Second:
		return value.Round(time.Millisecond).String()
	case value < time.Minute:
		return value.Round(100 * time.Millisecond).String()
	default:
		return value.Round(time.Second).String()
	}
}
