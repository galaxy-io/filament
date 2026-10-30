package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/streamkit"
)

// These adapters exercise the public integration contracts without importing a
// production connector or opening a database. Only certificates and committed
// destination state survive between attempts, never a source decoder or writer.
type layoutMemoryStore struct {
	*continuousTestStore
	codecs filament.CodecResolver
}

func (s *layoutMemoryStore) CommitEpoch(_ context.Context, commit filament.EpochCommit) (filament.CommittedEpoch, error) {
	if err := commit.ValidateCompletion(s.codecs); err != nil {
		return filament.CommittedEpoch{}, err
	}
	raw, err := commit.Certificate.CanonicalBytes(s.codecs)
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	var cert filament.EpochCertificate
	if err := json.Unmarshal(raw, &cert); err != nil {
		return filament.CommittedEpoch{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cert = &filament.CommittedEpoch{Certificate: cert, CommittedAt: time.Now()}
	key := cert.Ref.Key()
	s.state.LastEpoch = &key
	s.state.CommittedPositions = cert.Coverage.Positions.Clone()
	return *s.cert, nil
}

type layoutMemoryDestination struct {
	model     rowmodel.Schema
	emails    []*string
	committed int64
}

type layoutMemorySink struct {
	*continuousTestSink
	destination *layoutMemoryDestination
	policy      schema.Policy
	model       rowmodel.Schema
	emails      []*string
	applied     int
}

func (s *layoutMemorySink) SchemaPolicy() schema.Policy { return s.policy }
func (s *layoutMemorySink) Begin(ctx context.Context) error {
	s.model = s.destination.model.Clone()
	s.emails = nil
	return s.continuousTestSink.Begin(ctx)
}

func (s *layoutMemorySink) EnsureSchema(ctx context.Context, _ string, model rowmodel.Schema) error {
	if s.ref.Epoch == 0 {
		return errors.New("schema preparation outside epoch")
	}
	if len(s.model.Fields) == 0 {
		s.model = model.Clone()
		return nil
	}
	return schema.Reconcile(ctx, layoutMemoryTarget{&s.model}, model, s.policy)
}

func (s *layoutMemorySink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	expected := arrowbatch.Schema(s.model)
	if expected.NumFields() != int(b.Rows().NumCols()) {
		return filament.WriteReceipt{}, errors.New("write before schema preparation")
	}
	for _, field := range b.Rows().Schema().Fields() {
		indices := expected.FieldIndices(field.Name)
		if len(indices) != 1 || !reflect.DeepEqual(expected.Field(indices[0]), field) {
			return filament.WriteReceipt{}, errors.New("write layout differs from prepared model")
		}
	}
	// The pipeline writes named columns, so accept their order from the batch.
	names := b.Rows().Schema().FieldIndices("email")
	for i := 0; i < b.NumRows(); i++ {
		var value *string
		if len(names) > 0 && !b.Rows().Column(names[0]).IsNull(i) {
			v := b.Rows().Column(names[0]).ValueStr(i)
			value = &v
		}
		s.emails = append(s.emails, value)
	}
	s.applied += b.NumRows()
	return s.continuousTestSink.Apply(ctx, b, opts)
}

func (s *layoutMemorySink) Commit(ctx context.Context) error {
	err := s.continuousTestSink.Commit(ctx)
	if err != nil {
		return err
	}
	s.destination.model = s.model.Clone()
	s.destination.emails = append(s.destination.emails, s.emails...)
	s.destination.committed++
	return nil
}

func (s *layoutMemorySink) Abort(context.Context) error {
	s.model = rowmodel.Schema{}
	s.emails = nil
	return nil
}

type layoutMemoryTarget struct{ model *rowmodel.Schema }

func (t layoutMemoryTarget) Inspect(context.Context) (rowmodel.Schema, error) {
	return t.model.Clone(), nil
}

func (t layoutMemoryTarget) AddNullable(_ context.Context, fields []rowmodel.Field) error {
	t.model.Fields = append(t.model.Fields, fields...)
	return nil
}

type layoutMemorySource struct {
	*continuousTestSource
	epochs         [][]string
	layouts        *streamkit.MessageBatches
	destination    *layoutMemoryDestination
	next, position int
}

func (s *layoutMemorySource) OpenStream(context.Context, filament.StreamOpenOpts) (filament.StreamSession, error) {
	return nil, errors.New("runner skipped message negotiation")
}

func (s *layoutMemorySource) OpenMessageStream(_ context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	var err error
	s.layouts, err = streamkit.NewMessageBatches(opts, func(resource string) rowmodel.Schema { return rowmodel.Schema{Resource: resource} })
	if err != nil {
		return nil, err
	}
	for _, p := range opts.CommittedPositions {
		s.position, err = strconv.Atoi(string(p.Value))
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *layoutMemorySource) Read(ctx context.Context, out filament.StreamRecordSink, _ filament.Boundary) (filament.Coverage, error) {
	if s.next >= len(s.epochs) {
		return filament.Coverage{}, errors.New("unexpected read beyond supplied epochs")
	}
	payloads := make([][]byte, len(s.epochs[s.next]))
	for i, p := range s.epochs[s.next] {
		payloads[i] = []byte(p)
	}
	s.next++
	columns, w, err := s.layouts.Prepare(ctx, out, rowmodel.Schema{Resource: "events"}, payloads)
	if err != nil {
		return filament.Coverage{}, err
	}
	domain := filament.DomainKey{Domain: "log", Incarnation: "source"}
	projector := streamkit.NewEventMetadataProjector(w, s)
	var position filament.Position
	for _, payload := range payloads {
		write, err := columns.Prepare(payload)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.position++
		position = filament.Position{Codec: "opaque", Value: []byte(strconv.Itoa(s.position))}
		write(w)
		if err := projector.EndEvent(streamkit.Envelope{Identity: filament.EventIdentity{Domain: domain, Position: position}, KeyNull: true, Payload: payload}, rowmodel.Meta{}); err != nil {
			return filament.Coverage{}, err
		}
	}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: domain, Position: position}); err != nil {
		return filament.Coverage{}, err
	}
	return filament.Coverage{Positions: filament.DomainPositions{domain: position}}, nil
}

func (s *layoutMemorySource) Acknowledge(_ context.Context, coverage filament.Coverage) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.store.cert == nil || s.destination.committed != s.store.cert.Certificate.Ref.Epoch || !reflect.DeepEqual(coverage.Positions, s.store.cert.Certificate.Coverage.Positions) {
		return errors.New("acknowledgement before destination and checkpoint commit")
	}
	s.ack++
	return nil
}

func TestMessageSchemaConnectorIndependentLifecycle(t *testing.T) {
	for _, policy := range []schema.Policy{schema.NullableAdd, schema.Strict} {
		t.Run(string(policy), func(t *testing.T) {
			cfg, base, _, _ := continuousFixture(t)
			cfg.Boundary.MaxRecords = 2
			store := &layoutMemoryStore{continuousTestStore: base, codecs: cfg.Codecs}
			cfg.Store = store
			destination := &layoutMemoryDestination{}
			bus := inproc.New()
			defer bus.Close()
			subscription, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
			if err != nil {
				t.Fatal(err)
			}
			defer subscription.Close()
			cfg.Bus = bus
			attempt := 0
			run := func(epochs ...[]string) (*layoutMemorySource, *layoutMemorySink, error) {
				attempt++
				ref := *cfg.Spec.StreamAttempt
				ref.Token = int64(attempt)
				ref.ExecutionID = "attempt-" + strconv.Itoa(attempt)
				cfg.Spec.StreamAttempt = &ref
				store.mu.Lock()
				store.state.Attempt = &filament.Attempt{Lease: filament.LeaseToken{Tenant: cfg.Spec.Tenant, Attempt: ref}}
				store.mu.Unlock()
				source := &layoutMemorySource{continuousTestSource: &continuousTestSource{store: base}, epochs: epochs, destination: destination}
				sink := &layoutMemorySink{continuousTestSink: &continuousTestSink{}, destination: destination, policy: policy}
				cfg.Source, cfg.Sink, cfg.MaxEpochs = source, sink, len(epochs)
				return source, sink, RunContinuous(t.Context(), cfg)
			}
			initial := [][]string{{`{"name":"a"}`}, {`{"name":"b","email":"x"}`}}
			if policy == schema.Strict {
				initial = [][]string{{`{"name":"a"}`, `{"name":"b","email":"x"}`}}
			}
			source, _, err := run(initial...)
			if err != nil || source.ack != len(initial) {
				t.Fatalf("initial admission: %v acks=%d", err, source.ack)
			}
			if len(destination.emails) != 2 || destination.emails[0] != nil || destination.emails[1] == nil || *destination.emails[1] != "x" {
				t.Fatalf("late field values: %+v", destination.emails)
			}
			if len(store.cert.Certificate.MessageSchemas["events"].Fields) != 2 {
				t.Fatal("accepted layout not certified")
			}
			source, _, err = run([]string{`{"name":"c"}`})
			if err != nil || source.ack != 1 || len(destination.emails) != 3 || destination.emails[2] != nil {
				t.Fatalf("restart did not restore omitted field: %v", err)
			}
			before := store.cert.Certificate.Clone()
			bad := `{"email":42}`
			if policy == schema.Strict {
				bad = `{"new":true}`
			}
			source, sink, err := run([]string{`{"name":"valid"}`, bad})
			if !errors.Is(err, schema.ErrBlocked) || source.ack != 0 || sink.applied != 0 || len(destination.emails) != 3 {
				t.Fatalf("blocked batch had effects: %v acks=%d applied=%d", err, source.ack, sink.applied)
			}
			if !reflect.DeepEqual(before, store.cert.Certificate) {
				t.Fatal("blocked batch advanced checkpoint or layout")
			}
			assertLayoutLifecycleEvents(t, subscription, policy)
		})
	}
}

func assertLayoutLifecycleEvents(t *testing.T, sub eventbus.Subscription, policy schema.Policy) {
	t.Helper()
	attempts, detected, blocked := 0, 0, 0
	for attempts < 3 {
		select {
		case message := <-sub.C():
			fact, err := events.Decode(message)
			if err != nil {
				t.Fatal(err)
			}
			switch fact.Data.(type) {
			case events.StreamAttemptEndedEvent:
				attempts++
			case events.SchemaDetectedEvent:
				detected++
			case events.SchemaBlockedEvent:
				blocked++
			}
		case <-time.After(time.Second):
			t.Fatal("missing lifecycle events")
		}
	}
	if blocked != 1 || detected == 0 {
		t.Fatalf("missing diagnostics for %s: detected=%d blocked=%d", policy, detected, blocked)
	}
}
