package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/sync/singleflight"

	"github.com/galaxy-io/filament"
	workerv1 "github.com/galaxy-io/filament/api/worker/v1"
	"github.com/galaxy-io/filament/api/worker/v1/workerv1connect"
	"github.com/galaxy-io/filament/internal/convert"
)

const (
	// describeTTL bounds how long specs are served without asking the worker
	// again. They are fixed for a worker build, so this only matters across
	// an upgrade.
	describeTTL = 30 * time.Second
	// describeRetry is how long a failed refresh keeps serving the last
	// snapshot before the worker is asked again, so an outage costs one
	// timeout per interval rather than one per call.
	describeRetry = 5 * time.Second
	// describeTimeout bounds a spec fetch. The fetch is shared by every
	// waiting caller, so it runs detached from any one caller's deadline.
	describeTimeout = 5 * time.Second
	// callTimeout bounds a live call whose caller set no deadline, such as
	// the scheduler compiling on its process context.
	callTimeout = 30 * time.Second
)

// Remote returns a Worker that forwards to the persistent worker at baseURL,
// so the caller links no driver. Specs are served from a cached snapshot that
// refreshes in the background; every other call is one RPC under the
// caller's deadline. A bare host:port, as some platforms inject, is taken as
// plain HTTP.
func Remote(baseURL string, opts ...connect.ClientOption) filament.Worker {
	if !strings.Contains(baseURL, "://") {
		baseURL = "http://" + baseURL
	}
	return &remote{client: workerv1connect.NewWorkerServiceClient(http.DefaultClient, baseURL, opts...)}
}

type remote struct {
	client workerv1connect.WorkerServiceClient
	// flight shares one in-flight Describe among concurrent callers.
	flight singleflight.Group
	// mu guards the snapshot pointer. It is never held across a network call.
	mu       sync.Mutex
	snapshot *snapshot
}

var _ filament.Worker = (*remote)(nil)

// snapshot is one Describe of the worker: the catalog as listed, plus every
// name's concrete spec. An alias entry resolves to its target's spec.
type snapshot struct {
	at      time.Time
	catalog filament.Catalog
	sources map[string]filament.ConnectorSpec
	sinks   map[string]filament.SinkSpec
}

// wireError is a sentinel that crossed the wire: it matches the sentinel and
// keeps the worker's own message.
type wireError struct {
	sentinel error
	message  string
}

func (e wireError) Error() string { return e.message }
func (e wireError) Unwrap() error { return e.sentinel }

// fromConnect is the inverse of the handler's mapping. Codes without a
// sentinel keep the Connect error, so a down worker reads as unavailable.
func fromConnect(err error) error {
	var remote *connect.Error
	if !errors.As(err, &remote) {
		return err
	}
	switch remote.Code() {
	case connect.CodeNotFound:
		return wireError{sentinel: filament.ErrNotFound, message: remote.Message()}
	case connect.CodeUnimplemented:
		return wireError{sentinel: filament.ErrUnsupported, message: remote.Message()}
	case connect.CodeFailedPrecondition:
		return &filament.ConfigureError{Reason: errors.New(remote.Message())}
	default:
		return remote
	}
}

// described returns the current snapshot. A fresh one is returned as is. A
// stale one is returned too, with a refresh started behind it, so lookups
// never wait on the worker once a first snapshot exists. Only the very first
// lookup waits, and concurrent first lookups share one request.
func (r *remote) described(ctx context.Context) (*snapshot, error) {
	r.mu.Lock()
	snap := r.snapshot
	fresh := snap != nil && time.Since(snap.at) < describeTTL
	r.mu.Unlock()
	if fresh {
		return snap, nil
	}
	result := r.flight.DoChan("describe", func() (any, error) {
		callCtx, cancel := context.WithTimeout(context.Background(), describeTimeout)
		defer cancel()
		return r.refresh(callCtx)
	})
	if snap != nil {
		return snap, nil
	}
	select {
	case res := <-result:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.(*snapshot), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// refresh fetches the catalog and publishes it. A failed refresh keeps the
// last snapshot and backs off before the worker is asked again: the specs
// are still true of the worker that comes back.
func (r *remote) refresh(ctx context.Context) (*snapshot, error) {
	resp, err := r.client.Describe(ctx, connect.NewRequest(&workerv1.DescribeRequest{}))
	if err != nil {
		r.mu.Lock()
		if r.snapshot != nil {
			r.snapshot.at = time.Now().Add(describeRetry - describeTTL)
		}
		r.mu.Unlock()
		return nil, fromConnect(err)
	}
	next := &snapshot{at: time.Now(), sources: map[string]filament.ConnectorSpec{}, sinks: map[string]filament.SinkSpec{}}
	for _, source := range resp.Msg.GetSources() {
		spec, err := convert.SourceFromProto(source)
		if err != nil {
			return nil, fmt.Errorf("worker: describe: %w", err)
		}
		next.catalog.Sources = append(next.catalog.Sources, spec)
		next.sources[spec.Name] = spec
	}
	for _, sink := range resp.Msg.GetSinks() {
		spec, err := convert.SinkFromProto(sink)
		if err != nil {
			return nil, fmt.Errorf("worker: describe: %w", err)
		}
		next.catalog.Sinks = append(next.catalog.Sinks, spec)
		next.sinks[spec.Name] = spec
	}
	// A lookup by alias answers with the concrete spec, as the registry does.
	resolved := make(map[string]filament.ConnectorSpec, len(next.sources))
	for name, spec := range next.sources {
		if target, ok := next.sources[spec.AliasTarget]; ok && spec.AliasTarget != "" {
			spec = target
		}
		resolved[name] = spec
	}
	next.sources = resolved
	r.mu.Lock()
	r.snapshot = next
	r.mu.Unlock()
	return next, nil
}

func (r *remote) Describe(ctx context.Context) (filament.Catalog, error) {
	snap, err := r.described(ctx)
	if err != nil {
		return filament.Catalog{}, err
	}
	return snap.catalog, nil
}

func (r *remote) SourceSpec(ctx context.Context, name string) (filament.ConnectorSpec, error) {
	snap, err := r.described(ctx)
	if err != nil {
		return filament.ConnectorSpec{}, err
	}
	spec, ok := snap.sources[name]
	if !ok {
		return filament.ConnectorSpec{}, fmt.Errorf("%w: source %q", filament.ErrNotFound, name)
	}
	return spec, nil
}

func (r *remote) SinkSpec(ctx context.Context, name string) (filament.SinkSpec, error) {
	snap, err := r.described(ctx)
	if err != nil {
		return filament.SinkSpec{}, err
	}
	spec, ok := snap.sinks[name]
	if !ok {
		return filament.SinkSpec{}, fmt.Errorf("%w: sink %q", filament.ErrNotFound, name)
	}
	return spec, nil
}

func (r *remote) Validate(ctx context.Context, ref filament.ConnectorRef, cfg filament.Config) error {
	ctx, cancel := deadline(ctx)
	defer cancel()
	config, err := convert.StructFromMap(cfg.Raw())
	if err != nil {
		return err
	}
	_, err = r.client.Validate(ctx, connect.NewRequest(&workerv1.ValidateRequest{Connector: convert.RefToProto(ref), Config: config}))
	return fromConnect(err)
}

func (r *remote) TestConnection(ctx context.Context, ref filament.ConnectorRef, cfg filament.Config) error {
	ctx, cancel := deadline(ctx)
	defer cancel()
	config, err := convert.StructFromMap(cfg.Raw())
	if err != nil {
		return err
	}
	_, err = r.client.TestConnection(ctx, connect.NewRequest(&workerv1.TestConnectionRequest{Connector: convert.RefToProto(ref), Config: config}))
	return fromConnect(err)
}

func (r *remote) Discover(ctx context.Context, source string, cfg filament.Config, opts filament.DiscoverOpts) ([]filament.Resource, error) {
	ctx, cancel := deadline(ctx)
	defer cancel()
	config, err := convert.StructFromMap(cfg.Raw())
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Discover(ctx, connect.NewRequest(&workerv1.DiscoverRequest{Source: source, Config: config, Refresh: opts.Refresh}))
	if err != nil {
		return nil, fromConnect(err)
	}
	return convert.ResourcesFromProto(resp.Msg.GetResources()), nil
}

func (r *remote) Inspect(ctx context.Context, source string, cfg filament.Config, resources []string) ([]filament.Inspection, error) {
	ctx, cancel := deadline(ctx)
	defer cancel()
	config, err := convert.StructFromMap(cfg.Raw())
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Inspect(ctx, connect.NewRequest(&workerv1.InspectRequest{Source: source, Config: config, Resources: resources}))
	if err != nil {
		return nil, fromConnect(err)
	}
	return convert.InspectionsFromProto(resp.Msg.GetInspections())
}

func (r *remote) PlanReplicationStream(ctx context.Context, source string, req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	ctx, cancel := deadline(ctx)
	defer cancel()
	var raw map[string]any
	if req.Config != nil {
		raw = req.Config.Raw()
	}
	config, err := convert.StructFromMap(raw)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	resp, err := r.client.PlanReplicationStream(ctx, connect.NewRequest(&workerv1.PlanReplicationStreamRequest{
		Source:              source,
		SourceConnectionId:  req.SourceConnectionID,
		ReplicationStreamId: req.ReplicationStreamID,
		Config:              config,
		Resources:           req.Resources,
	}))
	if err != nil {
		return filament.ReplicationStreamPlan{}, fromConnect(err)
	}
	return convert.ReplicationStreamPlanFromProto(resp.Msg.GetPlan()), nil
}

// deadline applies callTimeout only when the caller set no deadline of its own.
func deadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, callTimeout)
}
