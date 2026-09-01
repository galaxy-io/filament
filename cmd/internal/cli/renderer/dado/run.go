package dado

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/atterpac/dado/inline"
	"golang.org/x/term"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
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

type resourceProgressState struct {
	records   int64
	bytes     int64
	estimated int64
	failed    bool
}

type runSummaryView struct {
	pipeline  string
	source    string
	sink      string
	resources int
	result    model.RunResult
	elapsed   time.Duration
}

func (summary runSummaryView) Frame(width int) *inline.Frame {
	theme := inline.RoundedInlineTheme()
	rows := [][2]string{
		{"Pipeline", summary.pipeline},
		{"Route", summary.source + " → " + summary.sink},
	}
	if summary.resources > 0 {
		rows = append(rows, [2]string{"Resources", humanCount(int64(summary.resources))})
	}
	rows = append(rows,
		[2]string{"Duration", humanDuration(summary.elapsed)},
		[2]string{"Records", humanCount(summary.result.Records)},
		[2]string{"Transferred", humanBytes(summary.result.Bytes)},
		[2]string{"Throughput", runThroughput(summary.result, summary.elapsed)},
	)

	frame := inline.NewFrame(width, len(rows)+2)
	frame.DrawString(0, 0, theme.Glyphs.Success+" Run completed", theme.Success.Bold(true))
	for index, row := range rows {
		y := index + 2
		frame.DrawString(0, y, row[0], theme.Muted)
		frame.DrawString(14, y, row[1], theme.Text)
	}
	return frame
}

func (r *Renderer) interactiveRun(ctx context.Context) error {
	listed, err := r.service.Pipelines(ctx)
	if err != nil {
		return err
	}
	if len(listed.Items) == 0 {
		return r.showInteractiveMessage(ctx, "Run a pipeline", "No saved pipelines. Create a source, sink, and pipeline first.")
	}
	options := make([]interactiveOption, 0, len(listed.Items)+1)
	for _, pipeline := range listed.Items {
		options = append(options, interactiveOption{label: fmt.Sprintf("%s  ·  %s → %s", pipeline.Name, pipeline.Source, pipeline.Sink), value: pipeline.Name})
	}
	options = append(options, interactiveOption{label: "Back", value: interactiveBack})
	name, err := r.chooseInteractive(ctx, "Run a pipeline", "Choose a saved pipeline", options)
	if err != nil || name == interactiveBack {
		return err
	}
	return r.runInteractivePipeline(ctx, name)
}

func (r *Renderer) runInteractivePipeline(ctx context.Context, name string) error {
	submission, err := r.service.PrepareRun(ctx, cliapp.RunRequest{Pipeline: name})
	if err != nil {
		return err
	}
	spec := submission.Spec
	doc, err := r.service.Configuration(ctx)
	if err != nil {
		return err
	}
	resources, _ := r.discoverPipelineResources(ctx, doc.Pipelines[name])
	if spec.Sink.Connector == "stdout" {
		if output, ok := r.stdout.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
			confirmed, confirmErr := r.confirmInteractive(ctx, "Run with the stdout sink?", "Records stream to stdout while progress renders on stderr. Redirect stdout for a clean display.")
			if confirmErr != nil || !confirmed {
				return confirmErr
			}
		}
	}
	startedAt := time.Now()
	result, err := r.renderInteractiveRun(ctx, submission, resources)
	if err != nil {
		return err
	}
	resourceCount := len(spec.Resources)
	if resourceCount == 0 {
		resourceCount = len(resources)
	}
	form := inline.NewForm("").SetHeader(runSummaryView{
		pipeline:  name,
		source:    spec.Source.Connector,
		sink:      spec.Sink.Connector,
		resources: resourceCount,
		result:    result,
		elapsed:   time.Since(startedAt),
	}).Add(inline.NewSelectField("selection", "", inline.NewChoice(interactiveBack, "Back")).Required())
	_, err = r.runInteractiveForm(ctx, form)
	return err
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
	progress := inline.NewMultiProgress(
		fmt.Sprintf("Filament run · %s → %s", spec.Source.Connector, spec.Sink.Connector),
		inline.WithCompletedTasksCollapsed(false),
		inline.WithAggregateProgress(true),
	)
	states := map[string]*resourceProgressState{}
	selected := make(map[string]bool, len(spec.Resources))
	for _, name := range spec.Resources {
		selected[name] = true
	}
	addResource := func(name string, estimated int64) error {
		if name == "" {
			return nil
		}
		if state, exists := states[name]; exists {
			if estimated > 0 && state.estimated == 0 {
				state.estimated = estimated
				return progress.SetTotal(name, estimated)
			}
			return nil
		}
		states[name] = &resourceProgressState{estimated: estimated}
		return progress.Add(name, name, estimated)
	}
	for _, resource := range discovered {
		if len(selected) == 0 || selected[resource.Name] {
			if err := addResource(resource.Name, resource.EstimatedRows); err != nil {
				return model.RunResult{}, err
			}
		}
	}
	for _, name := range spec.Resources {
		if err := addResource(name, 0); err != nil {
			return model.RunResult{}, err
		}
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

	width := interactiveRenderWidth(r.statusWriter())
	render := func() error { return r.interactiveRenderer.Render(progress.Frame(width)) }
	if err := render(); err != nil {
		return model.RunResult{}, err
	}
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case update := <-updates:
			if err := applyResourceProgress(progress, states, addResource, update); err != nil {
				return model.RunResult{}, err
			}
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case completed := <-done:
			for name, state := range states {
				if state.failed {
					continue
				}
				if completed.err == nil {
					_ = progress.Complete(name)
				} else {
					_ = progress.Cancel(name, "run stopped")
				}
			}
			return completed.result, errors.Join(completed.err, render())
		case <-ticker.C:
			if err := render(); err != nil {
				return model.RunResult{}, err
			}
		case <-ctx.Done():
			for name := range states {
				_ = progress.Cancel(name, "interrupted")
			}
			return model.RunResult{}, errors.Join(ctx.Err(), render())
		}
	}
}

func applyResourceProgress(
	progress *inline.MultiProgress,
	states map[string]*resourceProgressState,
	addResource func(string, int64) error,
	update resourceProgressUpdate,
) error {
	if err := addResource(update.resource, 0); err != nil {
		return err
	}
	state := states[update.resource]
	if state == nil {
		return nil
	}
	if update.final {
		state.records = update.records
		state.bytes = update.bytes
	} else {
		state.records += update.records
		state.bytes += update.bytes
	}
	detail := fmt.Sprintf("%d records · %s", state.records, humanBytes(state.bytes))
	if state.estimated > 0 {
		detail += fmt.Sprintf(" · %d estimated", state.estimated)
	}
	if err := progress.SetDetail(update.resource, detail); err != nil {
		return err
	}
	switch update.status {
	case "failed":
		state.failed = true
		return progress.Fail(update.resource, errors.New(update.err))
	case "complete":
		if state.estimated > 0 {
			_ = progress.Set(update.resource, state.records)
		}
		return progress.Complete(update.resource)
	default:
		if state.estimated > 0 {
			return progress.Set(update.resource, state.records)
		}
		return progress.Start(update.resource)
	}
}

func interactiveRenderWidth(writer io.Writer) int {
	if output, ok := writer.(*os.File); ok && term.IsTerminal(int(output.Fd())) {
		if width, _, err := term.GetSize(int(output.Fd())); err == nil && width > 1 {
			return width - 1
		}
	}
	return 96
}

func humanBytes(value int64) string {
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor, exponent := unit, 0
	for amount := value / unit; amount >= unit && exponent < 5; amount /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func humanCount(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	digits := strconv.FormatInt(value, 10)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	return sign + digits
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

func runThroughput(result model.RunResult, elapsed time.Duration) string {
	if elapsed <= 0 {
		return "—"
	}
	seconds := elapsed.Seconds()
	recordsPerSecond := int64(math.Round(float64(result.Records) / seconds))
	bytesPerSecond := int64(math.Round(float64(result.Bytes) / seconds))
	return fmt.Sprintf("%s records/s · %s/s", humanCount(recordsPerSecond), humanBytes(bytesPerSecond))
}
