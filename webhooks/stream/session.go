// Package stream turns one webhook endpoint's inbox into a continuous source
// session. A connector supplies how a batch of deliveries becomes rows. The
// package owns everything else: resuming after the certified position,
// batching, skipping resources the stream does not select, and coverage.
//
// The generic webhook source writes each delivery as an envelope or projected
// JSON columns. A connector that receives a provider's webhooks writes them as
// that provider's entities instead, over the same session.
package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

// Delivery is one accepted delivery and its identity in the stream.
type Delivery struct {
	inbox.Delivery
	Identity filament.EventIdentity
}

// WriteFunc emits rows for one batch of deliveries, in stream order. Every
// delivery's resource is one the stream selects. An error fails the session.
type WriteFunc func(ctx context.Context, out filament.StreamRecordSink, batch []Delivery) error

// Config is what a connector supplies to open a session.
type Config struct {
	Inbox *inbox.Inbox
	Batch Batch
	// Codecs canonicalizes committed positions. The connector registers the
	// inbox.PositionCodec it advertises.
	Codecs rowmodel.CodecResolver
	Write  WriteFunc
}

type session struct {
	cfg       Config
	reader    *inbox.Reader
	domain    filament.DomainKey
	source    string
	resources map[string]bool
	lifecycle stream.SourceLifecycle
}

// Open starts reading the endpoint opts.SourceConnectionID owns, after the
// position opts.CommittedPositions certified.
func Open(ctx context.Context, cfg Config, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if cfg.Inbox == nil || cfg.Codecs == nil || cfg.Write == nil || cfg.Batch.MaxRecords < 1 ||
		cfg.Batch.MaxBytes < 1 || cfg.Batch.MaxWait <= 0 {
		return nil, errors.New("webhook stream: inbox, codecs, writer, and batch limits required")
	}
	if opts.Tenant == "" || opts.SourceConnectionID == "" || opts.CheckAuthority == nil || len(opts.Resources) == 0 {
		return nil, errors.New("webhook stream: tenant, source, authority, and fixed resources required")
	}
	if err := opts.Attempt.Validate(); err != nil {
		return nil, err
	}
	if err := opts.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	resources := make(map[string]bool, len(opts.Resources))
	for _, resource := range opts.Resources {
		if resources[resource] {
			return nil, fmt.Errorf("webhook stream: repeated resource %q", resource)
		}
		resources[resource] = true
	}
	domain := inbox.Domain(opts.SourceConnectionID, cfg.Inbox.StreamName(), cfg.Inbox.Incarnation())
	certified, err := certifiedSequence(cfg.Codecs, domain, opts.CommittedPositions)
	if err != nil {
		return nil, err
	}
	lifecycle, err := stream.NewSourceLifecycle(opts.Attempt, opts.CheckAuthority)
	if err != nil {
		return nil, err
	}
	reader, err := cfg.Inbox.Reader(ctx, string(opts.Tenant), opts.SourceConnectionID, certified, cfg.Inbox.Incarnation())
	if err != nil {
		return nil, err
	}
	return &session{
		cfg: cfg, reader: reader, domain: domain, source: opts.SourceConnectionID,
		resources: resources, lifecycle: lifecycle,
	}, nil
}

// certifiedSequence reads the inbox sequence from certified progress. Any
// other domain means the progress belongs to a different inbox.
func certifiedSequence(codecs rowmodel.CodecResolver, domain filament.DomainKey, committed filament.DomainPositions) (uint64, error) {
	var certified uint64
	for key, position := range committed {
		if key != domain || position.Codec != inbox.PositionCodec || position.Version != 0 {
			return 0, filament.ErrPositionIncomparable
		}
		canonical, err := rowmodel.CanonicalPosition(codecs, position)
		if err != nil {
			return 0, err
		}
		if certified, err = strconv.ParseUint(string(canonical.Value), 10, 64); err != nil {
			return 0, err
		}
	}
	return certified, nil
}

// Read collects one batch and reports its last sequence as coverage. A
// delivery for a resource this stream does not select is skipped and logged,
// and still counts toward coverage, so it never blocks the endpoint.
func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, boundary filament.Boundary) (filament.Coverage, error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return filament.Coverage{}, err
	}
	if boundary.MaxWait <= 0 {
		return filament.Coverage{}, errors.New("webhook stream: positive idle wait required")
	}
	var selected []Delivery
	var last uint64
	var read int
	var bytes int64
	var deadline time.Time
	for read < s.cfg.Batch.MaxRecords && bytes < s.cfg.Batch.MaxBytes {
		wait := boundary.MaxWait
		if read > 0 {
			if wait = time.Until(deadline); wait <= 0 {
				break
			}
		}
		readCtx, cancel := context.WithTimeout(ctx, wait)
		d, seq, err := s.reader.Next(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout)) {
				break
			}
			s.lifecycle.Fail(err)
			return filament.Coverage{}, err
		}
		if read == 0 {
			deadline = time.Now().Add(s.cfg.Batch.MaxWait)
		}
		read++
		last = seq
		bytes += int64(len(d.Body))
		if !s.resources[d.Resource] {
			slog.WarnContext(ctx, "webhook delivery skipped: resource is not selected by this stream",
				"event.name", "webhook.delivery.skipped",
				"source_connection_id", s.source, "resource", d.Resource,
				"delivery_id", d.ID, "sequence", seq)
			continue
		}
		selected = append(selected, Delivery{
			Delivery: d,
			Identity: filament.EventIdentity{Domain: s.domain, Position: position(seq)},
		})
	}
	if read == 0 {
		return filament.Coverage{}, nil
	}
	if len(selected) > 0 {
		if err := s.cfg.Write(ctx, out, selected); err != nil {
			s.lifecycle.Fail(err)
			return filament.Coverage{}, err
		}
	}
	covered := position(last)
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: s.domain, Position: covered}); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	coverage := filament.Coverage{Positions: filament.DomainPositions{s.domain: covered}}
	if err := s.lifecycle.MarkRead(coverage); err != nil {
		return filament.Coverage{}, err
	}
	return coverage, nil
}

// Acknowledge records certified coverage. It takes no action in the inbox:
// the certificate is the progress, and retention purges what every consumer
// has certified. With no provider action there is no authority to check.
func (s *session) Acknowledge(_ context.Context, coverage filament.Coverage) error {
	if _, ok := coverage.Positions[s.domain]; !ok || len(coverage.Positions) != 1 {
		return filament.ErrIncompleteCoverage
	}
	return s.lifecycle.MarkAcknowledged(coverage)
}

func (s *session) Close(context.Context) error {
	s.reader.Close()
	s.lifecycle.MarkClosed(nil)
	return nil
}

func position(seq uint64) filament.Position {
	return filament.Position{Codec: inbox.PositionCodec, Value: []byte(strconv.FormatUint(seq, 10))}
}
