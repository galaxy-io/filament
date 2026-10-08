package export

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/request"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// JobCheckpoint is run-local, not an incremental watermark. Interrupted
// downloads replay the same artifact from its beginning (at-least-once).
// Signed URLs in Captures are sensitive checkpoint data and never logged.
type JobCheckpoint struct {
	Version  int               `json:"version"`
	Identity string            `json:"identity"`
	Phase    string            `json:"phase"` // creating | waiting | next | done
	Cursor   string            `json:"cursor,omitempty"`
	Deadline time.Time         `json:"deadline"`
	Captures map[string]string `json:"captures,omitempty"`
}

// Token serializes the existing version-one checkpoint format.
func (s JobCheckpoint) Token() string { b, _ := json.Marshal(s); return string(b) }

func (c *Runtime) runFull(ctx context.Context, res manifest.Resource, sink Sink) error {
	target := sink
	spec := res.Export
	scope := template.Scope{Config: c.Config, Env: c.Env}
	start, err := request.RenderResource(spec.Start.Resource(res.Name), scope)
	if err != nil {
		return err
	}
	state, err := c.exportState(res, start)
	if err != nil {
		return err
	}
	if state.Phase == "done" {
		return nil
	}
	ctx, cancel := context.WithDeadline(ctx, state.Deadline)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("export %s deadline: %w", res.Name, err)
	}
	// A located artifact is checkpointed before download, so recovery replays
	// the same file even after the locator has moved on.
	if spec.Locate != nil && len(state.Captures) == 0 {
		if state.Captures, err = c.locateExport(ctx, res, scope); err != nil {
			return err
		}
	}
	if spec.Direct && c.ResumeToken == "" {
		if err := target.Checkpoint(res.Name, state.Token()); err != nil {
			return err
		}
	}
	for state.Phase != "done" {
		progress := c.jobProgress(res.Name, state.Phase == "waiting" && c.ResumeToken != "")
		if state.Phase == "next" {
			scope.Job = state.Captures
			start, err = request.RenderResource(spec.Next.Request.Resource(res.Name), scope)
			if err != nil {
				return err
			}
			state.Cursor = state.Captures["next_cursor"]
			state.Phase = "creating"
		}
		if state.Phase == "creating" {
			// Publish intent first. This is a pipeline checkpoint marker, not an atomic
			// transaction with the upstream POST. Provider idempotency is still needed
			// to eliminate the crash window before the marker is durably acknowledged.
			if err := target.Checkpoint(res.Name, state.Token()); err != nil {
				return err
			}
			raw, err := c.Control.Start(ctx, start, scope)
			if err != nil {
				return fmt.Errorf("export %s creation outcome requires reconciliation: %w", res.Name, err)
			}
			state.Captures = map[string]string{}
			if err := captureExport(raw, spec.Start.Capture, state.Captures); err != nil {
				return fmt.Errorf("export %s start: %w", res.Name, err)
			}
			state.Phase = "waiting"
			progress.report(filament.SourceProgressExportJobCreated)
			if err := target.Checkpoint(res.Name, state.Token()); err != nil {
				return err
			}
		}
		scope.Job = state.Captures
		// Always retrieve job status again on resume, refreshing ephemeral URLs.
		if spec.Wait.Type == "job" {
			if err := c.waitExport(ctx, res, scope, progress); err != nil {
				return err
			}
			if err := target.Checkpoint(res.Name, state.Token()); err != nil {
				return err
			}
		}
		count, downloadBytes, err := c.consumeFullExport(ctx, res, sink, scope, state.Token(), progress)
		if err != nil {
			return err
		}
		state.Phase = "done"
		if spec.Next != nil && state.Captures["next_more"] == "true" {
			if state.Captures["next_cursor"] == state.Cursor {
				return fmt.Errorf("export %s: continuation cursor did not advance", res.Name)
			}
			state.Phase = "next"
		}
		// Completion is queued after all emitted rows, including for empty exports.
		if err := target.Checkpoint(res.Name, state.Token()); err != nil {
			return err
		}
		sink.Completed(count, downloadBytes)
	}
	return nil
}

// exportState validates recovery before any request can create or read a job.
func (c *Runtime) exportState(res, start manifest.Resource) (JobCheckpoint, error) {
	spec := res.Export
	config := c.IdentityConfig
	identityBytes, err := json.Marshal(struct {
		Resource manifest.Resource
		Start    manifest.Resource
		BaseURL  string
		Config   map[string]string
	}{res, start, c.BaseURL, config})
	if err != nil {
		return JobCheckpoint{}, err
	}
	identity := fmt.Sprintf("%x", sha256.Sum256(identityBytes))
	state := JobCheckpoint{Version: 1, Identity: identity, Phase: "creating", Deadline: time.Now().Add(time.Duration(spec.Wait.TimeoutSeconds) * time.Second)}
	if spec.Direct {
		state.Phase = "waiting"
	}
	token := c.ResumeToken
	if token != "" {
		state = JobCheckpoint{}
		if err := json.Unmarshal([]byte(token), &state); err != nil || state.Version != 1 || state.Identity != identity || state.Deadline.IsZero() {
			return JobCheckpoint{}, fmt.Errorf("export %s: incompatible checkpoint; cannot resume this job", res.Name)
		}
		switch state.Phase {
		case "done":
			return state, nil
		case "creating":
			return JobCheckpoint{}, fmt.Errorf("export %s: previous creation outcome is unknown; reconcile the upstream job before starting a fresh read", res.Name)
		case "next":
			if spec.Next == nil || state.Captures["next_more"] != "true" || state.Captures["next_cursor"] == "" {
				return JobCheckpoint{}, fmt.Errorf("export %s: invalid continuation checkpoint", res.Name)
			}
		case "waiting":
			if len(state.Captures) == 0 && !spec.Direct {
				return JobCheckpoint{}, fmt.Errorf("export %s: checkpoint has no job captures", res.Name)
			}
		default:
			return JobCheckpoint{}, fmt.Errorf("export %s: invalid checkpoint phase", res.Name)
		}
	}
	return state, nil
}

func exportPath(p string) string { return strings.TrimPrefix(p, "$.") }

func captureExport(raw []byte, captures, job map[string]string) error {
	for key, p := range captures {
		v := gjson.GetBytes(raw, exportPath(p))
		if !v.Exists() || v.Type == gjson.Null || v.IsObject() || v.IsArray() || v.String() == "" {
			return fmt.Errorf("missing or non-scalar job capture %q", key)
		}
		job[key] = v.String()
	}
	return nil
}

// locateExport reads the locator and captures the first group of each
// pattern's first match. A pattern without a match fails the read.
func (c *Runtime) locateExport(ctx context.Context, res manifest.Resource, scope template.Scope) (map[string]string, error) {
	if c.Locate == nil {
		return nil, fmt.Errorf("export %s: locator requests are unavailable", res.Name)
	}
	raw, err := c.Locate(ctx, res.Name, res.Export.Locate.Request, scope)
	if err != nil {
		return nil, err
	}
	captures := map[string]string{}
	for key, capture := range res.Export.Locate.Capture {
		match := regexp.MustCompile(capture.Regex).FindSubmatch(raw)
		if len(match) < 2 || len(match[1]) == 0 {
			return nil, fmt.Errorf("export %s: locator capture %q did not match", res.Name, key)
		}
		captures[key] = string(match[1])
	}
	return captures, nil
}

func (c *Runtime) waitExport(ctx context.Context, res manifest.Resource, scope template.Scope, progress *jobProgress) error {
	wait := res.Export.Wait
	for {
		raw, err := c.Control.Poll(ctx, res.Name, *wait.Request, scope)
		if err != nil {
			return err
		}
		progress.polled()
		v := gjson.GetBytes(raw, exportPath(wait.State.Path))
		if v.Type != gjson.String {
			return fmt.Errorf("export %s: missing or invalid job status", res.Name)
		}
		switch {
		case slices.Contains(wait.State.Ready, v.Str):
			if err := captureExport(raw, wait.Capture, scope.Job); err != nil {
				return err
			}
			if next := res.Export.Next; next != nil {
				more := gjson.GetBytes(raw, exportPath(next.MorePath))
				if more.Type != gjson.True && more.Type != gjson.False {
					return fmt.Errorf("export %s: missing or invalid continuation flag", res.Name)
				}
				scope.Job["next_more"] = more.String()
				delete(scope.Job, "next_cursor")
				if more.Bool() {
					if err := captureExport(raw, map[string]string{"next_cursor": next.CursorPath}, scope.Job); err != nil {
						return err
					}
				}
			}
			progress.ready()
			return nil
		case slices.Contains(wait.State.Failed, v.Str):
			return fmt.Errorf("export %s: job failed with status %q", res.Name, v.Str)
		case slices.Contains(wait.State.Pending, v.Str):
		default:
			return fmt.Errorf("export %s: unrecognized job status", res.Name)
		}
		if err := sleepCtx(ctx, time.Duration(wait.IntervalSeconds)*time.Second); err != nil {
			return err
		}
	}
}

func (c *Runtime) consumeFullExport(ctx context.Context, res manifest.Resource, sink Sink, scope template.Scope, checkpointToken string, progress *jobProgress) (int, int64, error) {
	location, err := template.Render(res.Export.Result.URL, scope)
	if err != nil {
		return 0, 0, fmt.Errorf("export result URL: %w", err)
	}
	resp, err := c.downloadExport(ctx, res, location, scope, progress)
	if err != nil {
		return 0, 0, err
	}

	var count int
	artifact := &artifactReader{Reader: resp.Body}
	err = decodeExport(ctx, artifact, res.Export.Result, func(row map[string]any) error {
		n, err := sink.Emit(row, nil, checkpointToken)
		count += n
		return err
	})
	_ = resp.Body.Close()
	if err != nil {
		return 0, 0, fmt.Errorf("export %s decode: %w", res.Name, err)
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	return count, artifact.bytes, nil
}
