package inbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
)

const PositionCodec = "webhook.inbox.sequence"

// Domain identifies the inbox position for one source and stream incarnation.
func Domain(sourceID, stream string, incarnation time.Time) filament.DomainKey {
	return filament.DomainKey{
		Domain: stream,
		Incarnation: fmt.Sprintf("%d:%s%d:%s%s", len(sourceID), sourceID,
			len(stream), stream, incarnation.UTC().Format(time.RFC3339Nano)),
	}
}

// Release says how far retention may purge one endpoint, before age is applied.
type Release struct {
	Tenant   string
	Endpoint string
	// Through is the highest sequence every resumable consumer has certified.
	Through uint64
	// Orphaned endpoints have no consumer that could resume. Age alone bounds them.
	Orphaned bool
}

// Policy bounds how long the inbox keeps deliveries.
type Policy struct {
	// Cutoff: a delivery accepted before it is purged once every consumer has
	// certified it. Keeping it until then lets sender retries deduplicate.
	Cutoff time.Time
	// HoldCutoff: a delivery accepted before it is purged even if no consumer
	// has certified it. It caps what a paused or stopped pipeline can hold.
	// It must not be after Cutoff.
	HoldCutoff time.Time
}

// Result reports one purge.
type Result struct {
	// Purged is about how many deliveries were removed.
	Purged uint64
	// Expired is about how many uncertified deliveries were dropped past the
	// hold, and ExpiredEndpoints lists the endpoints that lost them.
	Expired          uint64
	ExpiredEndpoints []Release
	// Oldest is when the oldest delivery still waiting for a consumer was
	// stored, zero when nothing is waiting. OldestEndpoint is its endpoint.
	Oldest         time.Time
	OldestEndpoint Release
}

// Purge removes released deliveries older than the cutoff and any delivery
// older than the hold. Its cost follows the number of endpoints, not the
// number of deliveries: a lookup and up to two purges each.
func (i *Inbox) Purge(ctx context.Context, policy Policy, releases []Release) (Result, error) {
	var result Result
	if i == nil || policy.Cutoff.IsZero() || policy.HoldCutoff.IsZero() || policy.HoldCutoff.After(policy.Cutoff) {
		return result, ErrInvalid
	}
	info, err := i.stream.Info(ctx)
	if err != nil {
		return result, err
	}
	if !info.Created.Equal(i.incarnation) {
		return result, errors.New("inbox: stream incarnation changed")
	}
	if info.State.Msgs == 0 {
		return result, nil
	}
	first, last := info.State.FirstSeq, info.State.LastSeq
	aged, err := i.firstAfter(ctx, policy.Cutoff, last)
	if err != nil {
		return result, err
	}
	held, err := i.firstAfter(ctx, policy.HoldCutoff, last)
	if err != nil {
		return result, err
	}
	for _, release := range releases {
		if release.Tenant == "" || release.Endpoint == "" {
			return Result{}, ErrInvalid
		}
		filter := endpointFilter(release.Tenant, release.Endpoint)
		if release.Orphaned {
			if err := i.purgeBefore(ctx, filter, aged, first); err != nil {
				return Result{}, err
			}
			continue
		}
		if err := i.purgeBefore(ctx, filter, min(aged, release.Through+1), first); err != nil {
			return Result{}, err
		}
		waiting, err := i.nextFor(ctx, filter, max(release.Through+1, first))
		if err != nil {
			return Result{}, err
		}
		if waiting != nil && waiting.Sequence < held {
			expired, err := i.countPurge(ctx, filter, held)
			if err != nil {
				return Result{}, err
			}
			result.Expired += expired
			result.ExpiredEndpoints = append(result.ExpiredEndpoints, release)
			if waiting, err = i.nextFor(ctx, filter, held); err != nil {
				return Result{}, err
			}
		}
		if waiting != nil && (result.Oldest.IsZero() || waiting.Time.Before(result.Oldest)) {
			result.Oldest, result.OldestEndpoint = waiting.Time, release
		}
	}
	after, err := i.stream.Info(ctx)
	if err != nil {
		return Result{}, err
	}
	result.Purged = info.State.Msgs - min(info.State.Msgs, after.State.Msgs)
	return result, nil
}

// purgeBefore removes an endpoint's deliveries below bound.
func (i *Inbox) purgeBefore(ctx context.Context, filter string, bound, first uint64) error {
	if bound <= first {
		return nil
	}
	if err := i.stream.Purge(ctx, jetstream.WithPurgeSubject(filter), jetstream.WithPurgeSequence(bound)); err != nil {
		return fmt.Errorf("inbox: purge endpoint: %w", err)
	}
	return nil
}

// countPurge removes an endpoint's deliveries below bound and reports about
// how many it removed. Concurrent acceptance can make the count low.
func (i *Inbox) countPurge(ctx context.Context, filter string, bound uint64) (uint64, error) {
	before, err := i.stream.Info(ctx)
	if err != nil {
		return 0, err
	}
	if err := i.stream.Purge(ctx, jetstream.WithPurgeSubject(filter), jetstream.WithPurgeSequence(bound)); err != nil {
		return 0, fmt.Errorf("inbox: expire endpoint: %w", err)
	}
	after, err := i.stream.Info(ctx)
	if err != nil {
		return 0, err
	}
	return before.State.Msgs - min(before.State.Msgs, after.State.Msgs), nil
}

// nextFor returns an endpoint's first delivery at or after seq, or nil.
func (i *Inbox) nextFor(ctx context.Context, filter string, seq uint64) (*jetstream.RawStreamMsg, error) {
	msg, err := i.stream.GetMsg(ctx, seq, jetstream.WithGetMsgSubject(filter))
	if errors.Is(err, jetstream.ErrMsgNotFound) {
		return nil, nil
	}
	return msg, err
}

// firstAfter returns the first sequence stored after cutoff, or last+1 when
// every message is older.
func (i *Inbox) firstAfter(ctx context.Context, cutoff time.Time, last uint64) (uint64, error) {
	newest, err := i.stream.GetMsg(ctx, last)
	if err != nil && !errors.Is(err, jetstream.ErrMsgNotFound) {
		return 0, err
	}
	if err == nil && !newest.Time.After(cutoff) {
		return last + 1, nil
	}
	start := cutoff.Add(time.Nanosecond)
	consumer, err := i.stream.OrderedConsumer(ctx, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{subjectRoot + ".>"}, HeadersOnly: true,
		DeliverPolicy: jetstream.DeliverByStartTimePolicy, OptStartTime: &start,
	})
	if err != nil {
		return 0, err
	}
	msg, err := consumer.Next(jetstream.FetchMaxWait(time.Second))
	if errors.Is(err, jetstream.ErrNoMessages) || errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return last + 1, nil
	}
	if err != nil {
		return 0, err
	}
	meta, err := msg.Metadata()
	if err != nil {
		return 0, err
	}
	return meta.Sequence.Stream, nil
}
