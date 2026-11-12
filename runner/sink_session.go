package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

// boundarySink owns destination sessions, independently of source epochs. A
// single-write connector receives a fresh instance and artifact identity for each
// boundary. The pipeline retains this stable forwarding sink across boundaries.
type boundarySink struct {
	sessionMu sync.RWMutex
	current   filament.Sink
	newSink   func() (filament.Sink, error)
	spec      filament.SinkSpec
	run       filament.RunSpec
	reusable  bool
	opened    bool
	closeErr  error
	schemaMu  sync.Mutex
	schemas   map[string]rowmodel.Schema
	policy    schema.Policy
}

func newBoundarySink(sink filament.Sink, factory func() (filament.Sink, error)) filament.Sink {
	s := &boundarySink{current: sink, newSink: factory, spec: sink.Spec(), reusable: sink.Spec().Capabilities.ReusableSession, schemas: make(map[string]rowmodel.Schema)}
	if _, ok := sink.(filament.SchemaPolicySink); ok {
		return &boundaryPolicySink{boundarySchemaSink: &boundarySchemaSink{s}}
	}
	if _, ok := sink.(filament.Schematized); ok {
		return &boundarySchemaSink{s}
	}
	return s
}
func (s *boundarySink) Name() string            { return s.spec.Name }
func (s *boundarySink) Spec() filament.SinkSpec { return s.spec }
func (s *boundarySink) Open(ctx context.Context, run filament.RunSpec) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	s.run = run
	return s.open(ctx)
}

func (s *boundarySink) open(ctx context.Context) error {
	run := s.run
	// Artifact identity and time belong to this finite write. Run identity and
	// its original start time remain available for logs, templates, and manifests.
	run.SinkWriteID = filament.RunID(uuid.NewString())
	run.SinkWriteStartedAt = time.Now().UTC()
	run.ExecutionID = ""
	run.StreamAttempt = nil
	run.ReplicationStream = nil
	run.Options.Execution = filament.ExecutionBounded
	s.opened = true // Close must clean up even a partially successful Open.
	if err := s.current.Open(ctx, run); err != nil {
		return err
	}
	if sink, ok := s.current.(filament.SchemaPolicySink); ok {
		s.policy = sink.SchemaPolicy()
	}
	return nil
}

func (s *boundarySink) Begin(ctx context.Context) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if s.closeErr != nil {
		return s.closeErr
	}
	if s.current == nil {
		if s.newSink == nil {
			return errors.New("sink requires a factory for repeated writes")
		}
		var err error
		s.current, err = s.newSink()
		if err != nil {
			return err
		}
		if s.current == nil {
			return errors.New("sink factory returned nil")
		}
		if err := s.open(ctx); err != nil {
			return err
		}
		if sink, ok := s.current.(filament.Schematized); ok {
			for resource, model := range s.schemas {
				if err := sink.EnsureSchema(ctx, resource, model.Clone()); err != nil {
					return fmt.Errorf("prepare sink boundary: %w", err)
				}
			}
		}
	}
	return s.current.Begin(ctx)
}

func (s *boundarySink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	return s.current.Apply(ctx, b, opts)
}

func (s *boundarySink) Commit(ctx context.Context) error {
	if err := s.current.Commit(ctx); err != nil {
		return err
	}
	if !s.reusable {
		return s.Close(ctx)
	}
	return nil
}

func (s *boundarySink) Abort(ctx context.Context) error {
	if s.current == nil {
		return nil
	}
	return s.current.Abort(ctx)
}

func (s *boundarySink) Close(ctx context.Context) error {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	if s.opened {
		s.closeErr = errors.Join(s.closeErr, s.current.Close(ctx))
		s.opened = false
		s.current = nil
	}
	return s.closeErr
}

type boundarySchemaSink struct{ *boundarySink }

func (s *boundarySchemaSink) EnsureSchema(ctx context.Context, resource string, model rowmodel.Schema) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Lifecycle transitions exclude schema calls, but independent DDL calls
	// share the read lock and can run concurrently.
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	if s.current != nil {
		if err := s.current.(filament.Schematized).EnsureSchema(ctx, resource, model); err != nil {
			return err
		}
	}
	// Between writes there is no live sink. Cache the schema for the next Begin.
	s.schemaMu.Lock()
	s.schemas[resource] = model.Clone()
	s.schemaMu.Unlock()
	return nil
}

type boundaryPolicySink struct{ *boundarySchemaSink }

func (s *boundaryPolicySink) SchemaPolicy() schema.Policy { return s.policy }
