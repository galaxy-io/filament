// Package local implements the CLI's local mode: the remote target pointed at
// the local engine, a YAML document applied into it, and untracked inline and
// override runs in a short-lived engine process.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/engine"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
)

var (
	_ cliapp.Target                 = (*Target)(nil)
	_ cliapp.RawConfigurationTarget = (*Target)(nil)
)

// Target is the local engine as the CLI sees it. Reads, writes, discovery,
// and saved-pipeline runs go to the engine's API; the document is an input
// applied into it; inline and override runs execute in engine.Inline.
type Target struct {
	*remote.Target
	store  Store
	engine *engine.Engine
	runMu  sync.Mutex
	runs   map[string]*runSession
}

// NewTarget wraps the engine's API. A nil engine refuses inline runs.
func NewTarget(store Store, deployment *remote.Target, local *engine.Engine) *Target {
	return &Target{Target: deployment, store: store, engine: local, runs: map[string]*runSession{}}
}

// SubmitRun sends saved pipelines to the engine's API and everything else,
// inline specs and override flags, to an untracked engine run.
func (t *Target) SubmitRun(ctx context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	if submission.Pipeline != nil && !submission.Override {
		return t.Target.SubmitRun(ctx, submission)
	}
	return t.submitLocal(ctx, submission.Spec)
}

// TailRun follows a run wherever it was submitted.
func (t *Target) TailRun(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	if len(group.Runs) == 1 && t.hasSession(group.Runs[0].ID) {
		return t.tailLocal(ctx, group, observe)
	}
	return t.Target.TailRun(ctx, group, observe)
}

// SignalRun signals a run wherever it was submitted.
func (t *Target) SignalRun(ctx context.Context, ref model.RunRef, signal filament.Signal) error {
	if t.hasSession(ref.ID) {
		return t.signalLocal(ref, signal)
	}
	return t.Target.SignalRun(ctx, ref, signal)
}

// ConfigurationLocation returns the document path.
func (t *Target) ConfigurationLocation() string { return t.store.Path }

// ReadConfiguration returns the exact document bytes, or an initialized
// document when the file does not exist.
func (t *Target) ReadConfiguration(context.Context) ([]byte, error) {
	data, err := os.ReadFile(t.store.Path)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(model.NewDocument()); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteConfiguration replaces the document and applies it.
func (t *Target) WriteConfiguration(ctx context.Context, data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse YAML tree: %w", err)
	}
	if err := t.store.Write(&root); err != nil {
		return err
	}
	return t.Apply(ctx)
}
