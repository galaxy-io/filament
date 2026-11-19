// Package alchemy resolves the Arrow contracts of alchemy functions and invokes them over record batches.
package alchemy

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/memory"

	functionsv1 "github.com/galaxy-io/alchemy/api/functions/v1"
	"github.com/galaxy-io/alchemy/api/functions/v1/functionsv1connect"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/transform"
)

const (
	// DefaultTimeout bounds one call, covering alchemy's own retries of a function.
	DefaultTimeout = 10 * time.Minute
	// lookupTimeout bounds one contract lookup, which only reads alchemy's catalog.
	lookupTimeout = 30 * time.Second
	// DefaultMaxPayloadBytes matches alchemy's default input and output cap.
	DefaultMaxPayloadBytes = 64 << 20
	// maxAttempts bounds sends of one input when alchemy is briefly unreachable.
	maxAttempts = 4
	// retryBase is the first retry's upper delay, doubling each attempt.
	retryBase = 250 * time.Millisecond
)

// Config configures a Client; only URL is required, and Credentials need Tenants.
type Config struct {
	URL             string
	HTTPClient      connect.HTTPClient
	Credentials     Credentials
	Tenants         filament.TenantDirectory
	Interceptors    []connect.Interceptor
	Timeout         time.Duration
	MaxPayloadBytes int
	Allocator       memory.Allocator
}

// Client calls one alchemy server.
type Client struct {
	fns        functionsv1connect.FunctionsServiceClient
	timeout    time.Duration
	maxPayload int
	mem        memory.Allocator
}

// New builds a Client from cfg, filling unset fields with defaults.
func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, errors.New("alchemy: URL is required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = DefaultMaxPayloadBytes
	}
	if cfg.Allocator == nil {
		cfg.Allocator = memory.DefaultAllocator
	}
	if cfg.Credentials != nil {
		if cfg.Tenants == nil {
			return nil, errors.New("alchemy: credentials need a tenant directory")
		}
		auth := &authenticator{creds: cfg.Credentials, tenants: cfg.Tenants}
		cfg.Interceptors = append([]connect.Interceptor{connect.UnaryInterceptorFunc(auth.intercept)}, cfg.Interceptors...)
	}
	return &Client{
		fns: functionsv1connect.NewFunctionsServiceClient(cfg.HTTPClient, cfg.URL,
			connect.WithInterceptors(cfg.Interceptors...), connect.WithReadMaxBytes(cfg.MaxPayloadBytes+(1<<20))),
		timeout:    cfg.Timeout,
		maxPayload: cfg.MaxPayloadBytes,
		mem:        cfg.Allocator,
	}, nil
}

var _ transform.FunctionService = (*Client)(nil)

// Functions exposes the raw service for catalog calls, scoped with WithTenant.
func (c *Client) Functions() functionsv1connect.FunctionsServiceClient { return c.fns }

// resolved is the outcome of resolving a set of refs, keeping why each unusable one cannot run.
type resolved struct {
	contracts transform.ContractMap
	errs      map[transform.FunctionRef]error
}

// Contract returns ref's contract or the reason it cannot run.
func (r *resolved) Contract(ref transform.FunctionRef) (transform.Contract, error) {
	if err, ok := r.errs[ref]; ok {
		return transform.Contract{}, fmt.Errorf("function %s: %w", ref, err)
	}
	return r.contracts.Contract(ref)
}

// Resolve fetches each ref's contract as tenant, recording per ref why one is missing, not ready, or not Arrow.
func (c *Client) Resolve(ctx context.Context, tenant filament.TenantID, refs []transform.FunctionRef) transform.Contracts {
	ctx = WithTenant(ctx, tenant)
	r := &resolved{contracts: make(transform.ContractMap, len(refs)), errs: map[transform.FunctionRef]error{}}
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, ref := range refs {
		wg.Go(func() {
			contract, err := c.contract(ctx, ref)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				r.errs[ref] = err
				return
			}
			r.contracts[ref] = contract
		})
	}
	wg.Wait()
	return r
}

func (c *Client) contract(ctx context.Context, ref transform.FunctionRef) (transform.Contract, error) {
	ctx, cancel := context.WithTimeout(ctx, min(c.timeout, lookupTimeout))
	defer cancel()
	resp, err := c.fns.GetFunctionVersion(ctx, connect.NewRequest(&functionsv1.GetFunctionVersionRequest{
		FunctionId: ref.ID,
		Version:    ref.Version,
	}))
	if err != nil {
		return transform.Contract{}, err
	}
	v := resp.Msg.GetVersion()
	switch v.GetStatus() {
	case functionsv1.FunctionVersionStatus_FUNCTION_VERSION_STATUS_READY:
	case functionsv1.FunctionVersionStatus_FUNCTION_VERSION_STATUS_FAILED:
		return transform.Contract{}, fmt.Errorf("version failed to build: %s", v.GetStatusMessage())
	default:
		return transform.Contract{}, errors.New("version is still building")
	}
	arrowContract := v.GetSpec().GetContract().GetArrow()
	if arrowContract == nil {
		return transform.Contract{}, errors.New("function has no Arrow contract; alchemy steps need one")
	}
	in, err := decodeSchema(arrowContract.GetInputSchema())
	if err != nil {
		return transform.Contract{}, fmt.Errorf("input schema: %w", err)
	}
	outSchema, err := decodeSchema(arrowContract.GetOutputSchema())
	if err != nil {
		return transform.Contract{}, fmt.Errorf("output schema: %w", err)
	}
	return transform.Contract{Input: in, Output: outSchema}, nil
}

// Invoke runs call.Function over in as tenant, splitting an input over the payload cap across calls.
func (c *Client) Invoke(ctx context.Context, tenant filament.TenantID, call transform.Call, in arrow.RecordBatch) (transform.Result, error) {
	ctx, cancel := context.WithTimeout(WithTenant(ctx, tenant), c.timeout)
	defer cancel()
	var res transform.Result
	rec, err := c.invokeSized(ctx, call.Function, in, &res.Invocations)
	if err != nil {
		return res, err
	}
	res.Record = rec
	return res, nil
}

// invokeSized sends in whole when it fits the cap, or halves it by rows until each part does.
func (c *Client) invokeSized(ctx context.Context, ref transform.FunctionRef, in arrow.RecordBatch, invs *[]transform.Invocation) (arrow.RecordBatch, error) {
	input, err := encodeBatch(in)
	if err != nil {
		return nil, fmt.Errorf("encode input: %w", err)
	}
	if len(input) <= c.maxPayload {
		inv, rec, err := c.invokeOnce(ctx, ref, input, in.NumRows())
		if err != nil {
			return nil, err
		}
		*invs = append(*invs, inv)
		return rec, nil
	}
	n := in.NumRows()
	if n <= 1 {
		return nil, fmt.Errorf("one row is %d bytes, over the %d byte limit", len(input), c.maxPayload)
	}
	var parts []arrow.RecordBatch
	defer func() {
		for _, part := range parts {
			part.Release()
		}
	}()
	for _, bounds := range [][2]int64{{0, n / 2}, {n / 2, n}} {
		half := in.NewSlice(bounds[0], bounds[1])
		out, err := c.invokeSized(ctx, ref, half, invs)
		half.Release()
		if err != nil {
			return nil, err
		}
		parts = append(parts, out)
	}
	return concatBatches(c.mem, parts)
}

// invokeOnce sends one input, retrying transient failures under one idempotency key so a retry replays, never reruns.
func (c *Client) invokeOnce(ctx context.Context, ref transform.FunctionRef, input []byte, rows int64) (transform.Invocation, arrow.RecordBatch, error) {
	started := time.Now()
	key := rand.Text()
	var resp *connect.Response[functionsv1.InvokeResponse]
	for attempt := 1; ; attempt++ {
		var err error
		resp, err = c.fns.Invoke(ctx, connect.NewRequest(&functionsv1.InvokeRequest{
			FunctionId:     ref.ID,
			Version:        ref.Version,
			Input:          input,
			IdempotencyKey: key,
		}))
		if err == nil {
			break
		}
		if attempt == maxAttempts || !retryable(err) {
			return transform.Invocation{}, nil, err
		}
		if waitErr := backoff(ctx, attempt); waitErr != nil {
			return transform.Invocation{}, nil, err
		}
	}
	msg := resp.Msg
	if fe := msg.GetError(); fe != nil {
		return transform.Invocation{}, nil, &transform.FunctionError{InvocationID: msg.GetInvocationId(), Kind: fe.GetKind().String(), Message: fe.GetMessage()}
	}
	if msg.GetVersion() != ref.Version {
		return transform.Invocation{}, nil, fmt.Errorf("alchemy ran version %d, want %d", msg.GetVersion(), ref.Version)
	}
	rec, err := decodeBatch(msg.GetOutput(), c.mem)
	if err != nil {
		return transform.Invocation{}, nil, fmt.Errorf("decode output of invocation %s: %w", msg.GetInvocationId(), err)
	}
	if rec.NumRows() != rows {
		rec.Release()
		return transform.Invocation{}, nil, fmt.Errorf("invocation %s returned %d rows, want %d", msg.GetInvocationId(), rec.NumRows(), rows)
	}
	return transform.Invocation{
		ID:       msg.GetInvocationId(),
		Records:  rows,
		Attempts: msg.GetAttempts(),
		Replayed: msg.GetReplayed(),
		Duration: time.Since(started),
	}, rec, nil
}

// retryable reports a failure that never reached the function or that alchemy asks to retry.
func retryable(err error) bool {
	switch connect.CodeOf(err) {
	case connect.CodeUnavailable, connect.CodeResourceExhausted, connect.CodeAborted:
		return true
	}
	return false
}

// backoff waits a jittered, doubling delay before the next attempt.
func backoff(ctx context.Context, attempt int) error {
	t := time.NewTimer(mrand.N(retryBase << attempt)) //nolint:gosec // jitter, not a secret
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
