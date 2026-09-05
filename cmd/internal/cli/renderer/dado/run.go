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
	route    string
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
	var listed model.PipelineList
	if err := r.loading("Loading pipelines…", func() (err error) {
		listed, err = r.service.Pipelines(ctx)
		return err
	}); err != nil {
		return err
	}
	if len(listed.Items) == 0 {
		return r.notice(false, "No saved pipelines. Create a source, sink, and pipeline first.")
	}
	name, err := r.chooseInteractive(ctx, "Run a pipeline", "Choose a saved pipeline", r.pipelineMenuOptions(listed.Items))
	if err != nil {
		return err
	}
	return r.runInteractivePipeline(ctx, name)
}

// runInteractivePipeline runs a chosen pipeline. Inside the menus the run
// is a screen that stays up until dismissed and then toasts its outcome; as
// a one-shot operation the table and summary stay in scrollback.
func (r *Renderer) runInteractivePipeline(ctx context.Context, name string) error {
	return r.runRequest(ctx, cliapp.RunRequest{Pipeline: name}, r.menuMode)
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
	if err := r.runRequest(ctx, request, false); err != nil && !interactiveInterrupted(err) {
		return err
	}
	return nil
}

// runRequest prepares and renders one run. A transient run keeps its title
// and discovery line inside the live frame and leaves nothing in scrollback;
// otherwise they print above the frame and the final rows stay behind.
func (r *Renderer) runRequest(ctx context.Context, request cliapp.RunRequest, transient bool) error {
	var submission model.RunSubmission
	var resources []model.ResourceSummary
	var discoverErr error
	if err := r.loading("Preparing run…", func() error {
		var err error
		if submission, err = r.service.PrepareRun(ctx, request); err != nil {
			return err
		}
		if request.Pipeline == "" {
			return nil
		}
		doc, err := r.service.Configuration(ctx)
		if err != nil {
			return err
		}
		resources, discoverErr = r.discoverPipelineResources(ctx, doc.Pipelines[request.Pipeline])
		return nil
	}); err != nil {
		return err
	}
	spec := submission.Spec
	view := &runProgressView{theme: r.theme, title: runRoute(request.Pipeline, spec)}
	if request.Pipeline != "" {
		view.note = "Discovering resources… " + discoveryWord(discoverErr)
	}
	if spec.Sink.Connector == "stdout" {
		if output, ok := r.stdout.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
			confirmed, confirmErr := r.confirmInteractive(ctx, "Run with the stdout sink?", "Records stream to stdout while progress renders on stderr. Redirect stdout for a clean display.")
			if confirmErr != nil || !confirmed {
				return confirmErr
			}
		}
	}
	_, err := r.renderInteractiveRun(ctx, view, submission, resources, transient)
	return err
}

// runRoute titles a run by its pipeline name, or by connectors when inline.
func runRoute(name string, spec filament.RunSpec) string {
	if name != "" {
		return name
	}
	return spec.Source.Connector + " → " + spec.Sink.Connector
}

func discoveryWord(discoverErr error) string {
	if discoverErr != nil {
		return "Unavailable"
	}
	return "Ok"
}

func (r *Renderer) renderInteractiveRun(ctx context.Context, view *runProgressView, submission model.RunSubmission, discovered []model.ResourceSummary, transient bool) (model.RunResult, error) {
	if r.interactiveRenderer == nil {
		return model.RunResult{}, errors.New("interactive renderer is not running")
	}
	spec := submission.Spec
	group, err := r.service.SubmitRun(ctx, submission)
	if err != nil {
		return model.RunResult{}, err
	}
	rows := map[string]*runRow{}
	addResource := func(key, name string) {
		if key == "" || rows[key] != nil {
			return
		}
		row := &runRow{name: name}
		rows[key] = row
		view.rows = append(view.rows, row)
	}
	selected := make(map[string]bool, len(spec.Resources))
	for _, name := range spec.Resources {
		selected[name] = true
	}
	if len(group.Runs) == 1 {
		for _, resource := range discovered {
			if len(selected) == 0 || selected[resource.Name] {
				addResource(resource.Name, resource.Name)
			}
		}
		for _, name := range spec.Resources {
			addResource(name, name)
		}
	}

	updates := make(chan resourceProgressUpdate, 256)
	done := make(chan runProgressDone, 1)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		result, runErr := r.service.TailRun(runCtx, group, func(event model.RunEvent) {
			update := resourceProgressUpdate{
				route: event.Route, resource: event.Resource, status: event.Status, records: event.Records,
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
			key, label := runResourceKey(update, len(group.Runs) > 1)
			addResource(key, label)
			applyRunUpdate(rows[key], update)
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case completed := <-done:
			drainRunUpdates(updates, rows, addResource, len(group.Runs) > 1)
			settleRows(view, completed.result, completed.err)
			return completed.result, errors.Join(completed.err, r.finishRun(ctx, view, completed.result, time.Since(startedAt), completed.err, transient))
		case <-ticker.C:
			view.tick++
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case <-ctx.Done():
			settleRows(view, model.RunResult{Status: "canceled"}, ctx.Err())
			return model.RunResult{}, errors.Join(ctx.Err(), r.finishRun(ctx, view, model.RunResult{}, time.Since(startedAt), ctx.Err(), transient))
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
func drainRunUpdates(updates <-chan resourceProgressUpdate, rows map[string]*runRow, addResource func(string, string), multipleRoutes bool) {
	for {
		select {
		case update := <-updates:
			key, label := runResourceKey(update, multipleRoutes)
			addResource(key, label)
			applyRunUpdate(rows[key], update)
		default:
			return
		}
	}
}

// settleRows resolves rows still in flight once the run has ended.
func settleRows(view *runProgressView, result model.RunResult, runErr error) {
	for _, row := range view.rows {
		if row.state != rowPending && row.state != rowRunning {
			continue
		}
		switch {
		case result.Status == "complete" && runErr == nil:
			row.state = rowDone
		case result.Status == "failed" || result.Status == "partial":
			row.state = rowFailed
			if row.err == "" {
				row.err = result.Status
			}
		default:
			row.state = rowCancelled
		}
	}
}

func runResourceKey(update resourceProgressUpdate, multipleRoutes bool) (string, string) {
	if !multipleRoutes || update.route == "" {
		return update.resource, update.resource
	}
	return update.route + "\x00" + update.resource, update.route + " · " + update.resource
}

// finishRun ends the live frame. Inside the menus the final rows stay on
// screen until the user goes back, then the outcome becomes the next menu's
// toast. As a one-shot operation the rows and the summary move into
// scrollback and the command exits.
func (r *Renderer) finishRun(ctx context.Context, view *runProgressView, result model.RunResult, elapsed time.Duration, runErr error, transient bool) error {
	if err := r.interactiveRenderer.Clear(); err != nil {
		return err
	}
	if transient {
		plain := style.Painter{}
		summary := runSummaryLine(plain, len(view.rows), result, elapsed, runErr)
		pairs := [][2]string{}
		if view.note != "" {
			pairs = append(pairs, [2]string{"", view.note}, [2]string{"", ""})
		}
		for line := range strings.SplitSeq(strings.TrimRight(view.lines(plain), "\n"), "\n") {
			pairs = append(pairs, [2]string{"", strings.TrimPrefix(line, style.Indent)})
		}
		pairs = append(pairs, [2]string{"", ""}, [2]string{"", summary})
		if err := r.showDetail(ctx, view.title, pairs); err != nil && !interactiveCancelled(err) && !interactiveInterrupted(err) {
			return err
		}
		return r.notice(result.Status == "complete" && runErr == nil, view.title+": "+summary)
	}
	p := r.painter()
	if err := r.interactiveRenderer.Println(p.Title("Ran", view.title) + "\n" + strings.TrimRight(view.lines(p), "\n")); err != nil {
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
