package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/streamkit"
)

var _ filament.StreamSource = (*Source)(nil)

// OpenStream opens one persistent pgoutput session. An uncertified initial
// snapshot completes before the session returns its first candidate coverage.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (_ filament.StreamSession, result error) {
	lifecycle, err := stream.NewSourceLifecycle(opts.Attempt, opts.CheckAuthority)
	if err != nil {
		return nil, err
	}
	// Continuous workers carry admitted identity directly. Derive the same
	// consumer name as planning, rather than sharing the bounded default slot.
	plan, err := s.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{ReplicationStreamID: opts.Attempt.StreamID, Config: filament.NewConfig(nil)})
	if err != nil {
		return nil, err
	}
	if s.slotName != defaultSlotName && s.slotName != plan.ConsumerName {
		return nil, errors.New("postgres continuous: configured slot does not match admitted stream")
	}
	bound := *s
	bound.slotName = plan.ConsumerName
	s = &bound
	if s.pool == nil || len(opts.Resources) == 0 {
		return nil, errors.New("postgres continuous: configured source and resources required")
	}
	if s.snapshotMode != filament.SnapshotNone && s.snapshotMode != filament.SnapshotInitial {
		return nil, errors.New("postgres continuous: unsupported snapshot mode")
	}
	if err := lifecycle.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	tracked := map[string]*cdcTable{}
	for _, resource := range opts.Resources {
		if _, exists := tracked[resource]; exists {
			return nil, errors.New("postgres continuous: duplicate resource")
		}
		model, err := s.Schema(ctx, resource)
		if err != nil {
			return nil, err
		}
		if len(model.PrimaryKey) == 0 {
			return nil, fmt.Errorf("postgres continuous: %s requires a primary key", resource)
		}
		tracked[resource] = &cdcTable{schema: model, pks: model.PrimaryKey}
	}
	if err := s.ensurePublication(ctx, opts.Resources); err != nil {
		return nil, err
	}
	bootstrap, err := s.planContinuousSnapshot(ctx, opts)
	if err != nil {
		return nil, err
	}
	conn, err := s.replicationConn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			bootstrap.close(cleanup)
			_ = conn.Close(cleanup)
		}
	}()
	system, err := pglogrepl.IdentifySystem(ctx, conn)
	if err != nil {
		return nil, err
	}
	domain := filament.DomainKey{Domain: "postgres:" + s.slotName, Incarnation: fmt.Sprintf("%s/%d/%s", system.SystemID, system.Timeline, system.DBName)}
	start, slot, err := s.resumeStream(ctx, conn, domain, opts.CommittedPositions)
	if err != nil {
		return nil, err
	}
	if bootstrap != nil {
		if err := bootstrap.open(ctx, slot); err != nil {
			return nil, err
		}
	}
	if err := lifecycle.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	if err := pglogrepl.StartReplication(ctx, conn, s.slotName, start, pglogrepl.StartReplicationOptions{PluginArgs: []string{"proto_version '1'", "publication_names '" + s.publication + "'"}}); err != nil {
		return nil, err
	}
	pumpCtx, cancel := context.WithCancel(ctx)
	session := &walSession{
		conn: conn, domain: domain, lifecycle: lifecycle, start: start, acknowledged: slot.start, bootstrap: bootstrap, cancel: cancel, events: make(chan walEvent, 64), acks: make(chan walAck), done: make(chan struct{}), checkAuthority: opts.CheckAuthority,
		decoder: &pgCDCRun{schema: s.schema, resources: append([]string(nil), opts.Resources...), tracked: tracked, relations: map[uint32]*cdcRelation{}},
	}
	go session.pump(pumpCtx)
	return session, nil
}

type walEvent struct {
	data     []byte
	start    pglogrepl.LSN
	progress pglogrepl.LSN
}
type walAck struct {
	ctx    context.Context
	target pglogrepl.LSN
	done   chan error
}
type walSession struct {
	conn           *pgconn.PgConn
	domain         filament.DomainKey
	lifecycle      stream.SourceLifecycle
	decoder        *pgCDCRun
	start          pglogrepl.LSN
	acknowledged   pglogrepl.LSN // owned exclusively by pump
	checkAuthority func(context.Context) error
	cancel         context.CancelFunc
	events         chan walEvent
	acks           chan walAck
	done           chan struct{}
	pumpErr        error // read only after done closes
	bootstrap      *continuousSnapshot
	txn            string
	skip           bool
}

// Read cuts a batch only after a complete source transaction. Row/byte/age
// targets are soft: a large transaction is never split across sink commits.
func (s *walSession) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (coverage filament.Coverage, result error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return coverage, err
	}
	if b.MaxRecords <= 0 || b.MaxWait <= 0 || b.MaxAge < 0 || b.MaxBytes < 0 {
		return coverage, errors.New("postgres continuous: invalid boundary")
	}
	defer func() {
		if result != nil {
			s.lifecycle.Fail(result)
		}
	}()
	s.decoder.sink = out
	// Snapshot and WAL use the same session-long inlet and writer cache.
	tracker, err := streamkit.NewBoundaryTracker(b)
	if err != nil {
		return coverage, err
	}
	defer tracker.Close()
	var end, bootstrapTarget pglogrepl.LSN
	if s.bootstrap != nil {
		bootstrapTarget = s.bootstrap.target
		if err := s.readSnapshot(ctx, out); err != nil {
			return coverage, err
		}
		end = s.start
	}
	finish := func() (filament.Coverage, error) {
		if end == 0 {
			return filament.Coverage{}, nil
		}
		if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: s.domain, Position: walPosition(end)}); err != nil {
			return filament.Coverage{}, err
		}
		c := filament.Coverage{Positions: filament.DomainPositions{s.domain: walPosition(end)}}
		return c, s.lifecycle.MarkRead(c)
	}
	for {
		if s.txn == "" && (bootstrapTarget != 0 && end >= bootstrapTarget || bootstrapTarget == 0 && tracker.Requested()) {
			return finish()
		}
		select {
		case <-ctx.Done():
			return coverage, ctx.Err()
		case <-tracker.Wake():
			// A snapshot retry must catch up to its consistent point even after a
			// soft deadline. Otherwise old WAL could overwrite the new baseline.
		case <-s.done:
			return coverage, s.pumpFailure()
		case event := <-s.events:
			if len(event.data) == 0 {
				if bootstrapTarget != 0 && s.txn == "" {
					end = max(end, event.progress)
				}
				continue
			}
			before := s.decoder.emitted
			completed, err := s.decodeEvent(ctx, out, event)
			if err != nil {
				return coverage, err
			}
			end = max(end, completed)
			if rows := s.decoder.emitted - before; rows > 0 {
				if err := tracker.Add(rows, int64(len(event.data))); err != nil {
					return coverage, err
				}
			}
		}
	}
}

// checkRelation rejects layout drift rather than silently parsing WAL into an
// obsolete Arrow schema. Restart with a reviewed schema after such a failure.
func (s *walSession) checkRelation(m *pglogrepl.RelationMessage) error {
	table, ok := s.decoder.tracked[m.RelationName]
	if !ok || m.Namespace != s.decoder.schema {
		return nil
	}
	if previous := s.decoder.relations[m.RelationID]; previous != nil && !reflect.DeepEqual(previous.message, m) {
		return errors.New("postgres continuous: relation changed during session")
	}
	if len(m.Columns) != len(table.schema.Fields) {
		return errors.New("postgres continuous: relation column count changed")
	}
	for _, f := range table.schema.Fields {
		found := false
		for _, c := range m.Columns {
			if c.Name == f.Name {
				typ, _ := typeFor(c.DataType, f)
				if typ.logical != f.Logical {
					return fmt.Errorf("postgres continuous: type changed for %s.%s", m.RelationName, f.Name)
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("postgres continuous: column %s missing", f.Name)
		}
	}
	return nil
}

func (s *walSession) Acknowledge(ctx context.Context, c filament.Coverage) error {
	if err := s.lifecycle.CheckAcknowledge(ctx, c); err != nil {
		return err
	}
	target, err := pglogrepl.ParseLSN(string(c.Positions[s.domain].Value))
	if err != nil {
		return err
	}
	request := walAck{ctx: ctx, target: target, done: make(chan error, 1)}
	select {
	case s.acks <- request:
	case <-s.done:
		return s.pumpFailure()
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.done:
		if err != nil {
			s.lifecycle.Fail(err)
			return err
		}
		return s.lifecycle.MarkAcknowledged(c)
	case <-s.done:
		return s.pumpFailure()
	case <-ctx.Done():
		s.lifecycle.Fail(ctx.Err())
		return ctx.Err()
	}
}

func (s *walSession) pumpFailure() error {
	if s.pumpErr != nil {
		return s.pumpErr
	}
	return errors.New("postgres continuous: replication session closed")
}

func (s *walSession) Close(ctx context.Context) error {
	if closed, err := s.lifecycle.Closed(); closed {
		return err
	}
	s.cancel()
	s.bootstrap.close(ctx)
	select {
	case <-s.done:
		err := s.pumpErr
		s.lifecycle.MarkClosed(err)
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// resumeStream checks certified progress against retained WAL before starting I/O.
func (s *Source) resumeStream(ctx context.Context, conn *pgconn.PgConn, domain filament.DomainKey, positions filament.DomainPositions) (pglogrepl.LSN, replicationSlotState, error) {
	var resume pglogrepl.LSN
	var err error
	for d, p := range positions {
		if d != domain {
			return 0, replicationSlotState{}, filament.ErrPositionIncomparable
		}
		if err := (LSNCodec{}).Validate(p); err != nil {
			return 0, replicationSlotState{}, err
		}
		resume, err = pglogrepl.ParseLSN(string(p.Value))
		if err != nil {
			return 0, replicationSlotState{}, err
		}
	}
	if len(positions) > 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_replication_slots WHERE slot_name=$1)", s.slotName).Scan(&exists); err != nil {
			return 0, replicationSlotState{}, err
		}
		if !exists {
			return 0, replicationSlotState{}, errors.New("postgres continuous: certified slot is missing; refusing to skip retained WAL")
		}
	}
	slot, err := s.ensureReplicationSlot(ctx, conn)
	if err != nil {
		return 0, replicationSlotState{}, err
	}
	start := slot.start
	if len(positions) > 0 {
		if resume < slot.start {
			return 0, replicationSlotState{}, fmt.Errorf("postgres continuous: certified position %s is behind slot %s", resume, slot.start)
		}
		start = resume
	}
	current, err := s.currentLSN(ctx)
	if err != nil {
		return 0, replicationSlotState{}, err
	}
	if start > current {
		return 0, replicationSlotState{}, errors.New("postgres continuous: resume position is ahead of server WAL")
	}

	return start, slot, nil
}

// decodeEvent preserves transaction controls and validates layouts before rows.
func (s *walSession) decodeEvent(ctx context.Context, out filament.StreamRecordSink, event walEvent) (pglogrepl.LSN, error) {
	var end pglogrepl.LSN
	msg, err := pglogrepl.Parse(event.data)
	if err != nil {
		return 0, err
	}
	switch m := msg.(type) {
	case *pglogrepl.BeginMessage:
		if s.txn != "" {
			return 0, errors.New("postgres continuous: nested transaction")
		}
		s.txn = strconv.FormatUint(uint64(m.Xid), 10)
		s.skip = m.FinalLSN < s.start
		if !s.skip {
			if err := out.Control(ctx, filament.Control{Kind: filament.TxnBegin, Domain: s.domain, Position: walPosition(m.FinalLSN), TxnID: s.txn}); err != nil {
				return 0, err
			}
		}
	case *pglogrepl.RelationMessage:
		if err := s.checkRelation(m); err != nil {
			return 0, err
		}
	case *pglogrepl.CommitMessage:
		if s.txn == "" {
			return 0, errors.New("postgres continuous: commit without begin")
		}
		if !s.skip {
			if err := out.Control(ctx, filament.Control{Kind: filament.TxnEnd, Domain: s.domain, Position: walPosition(m.TransactionEndLSN), TxnID: s.txn}); err != nil {
				return 0, err
			}
			end = m.TransactionEndLSN
		}
		s.txn = ""
	}
	if _, ok := msg.(*pglogrepl.RelationMessage); ok || !s.skip {
		if _, err := s.decoder.process(event.data, event.start); err != nil {
			return 0, err
		}
	}
	return end, nil
}

// validateContinuousConfig is pure admission shared by API validation and compilation.
func validateContinuousConfig(cfg filament.Config, resources []string) error {
	if cfg.String("replication") != replicationCDC {
		return errors.New("postgres continuous requires replication=cdc")
	}
	switch filament.SnapshotMode(cfg.String("snapshot_mode")) {
	case "", filament.SnapshotNone, filament.SnapshotInitial:
	default:
		return errors.New("postgres continuous: unsupported snapshot mode")
	}
	if len(resources) == 0 {
		return errors.New("postgres continuous requires explicit table selection")
	}
	seen := make(map[string]bool, len(resources))
	for _, resource := range resources {
		if resource == "" || seen[resource] {
			return errors.New("postgres continuous requires distinct, nonempty table names")
		}
		seen[resource] = true
	}
	return nil
}
