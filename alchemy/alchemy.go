// Package alchemy resolves the Arrow contracts of alchemy functions and invokes them over record batches.
package alchemy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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

// Functions exposes the raw service for catalog calls.
func (c *Client) Functions() functionsv1connect.FunctionsServiceClient { return c.fns }

// Resolved is the outcome of resolving a set of refs, keeping why each unusable one cannot run.
type Resolved struct {
	refs      []transform.FunctionRef
	contracts transform.ContractMap
	errs      map[transform.FunctionRef]error
}

var _ transform.Contracts = (*Resolved)(nil)

// Contract returns ref's contract or the reason it cannot run.
func (r *Resolved) Contract(ref transform.FunctionRef) (transform.Contract, error) {
	if err, ok := r.errs[ref]; ok {
		return transform.Contract{}, fmt.Errorf("function %s: %w", ref, err)
	}
	return r.contracts.Contract(ref)
}

// Err is the first ref, in request order, that cannot run.
func (r *Resolved) Err() error {
	for _, ref := range r.refs {
		if _, ok := r.errs[ref]; ok {
			_, err := r.Contract(ref)
			return err
		}
	}
	return nil
}

// Resolve fetches each ref's contract, recording per ref why one is missing, not ready, or not Arrow.
func (c *Client) Resolve(ctx context.Context, refs []transform.FunctionRef) *Resolved {
	r := &Resolved{refs: refs, contracts: make(transform.ContractMap, len(refs)), errs: map[transform.FunctionRef]error{}}
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

// Request is one invocation; Scope namespaces its idempotency key, which also covers the input bytes.
type Request struct {
	Function transform.FunctionRef
	Scope    string
}

// Result is a function's output and alchemy's record of the call; the caller releases Record.
type Result struct {
	Record       arrow.RecordBatch
	InvocationID string
	Attempts     int32
	Replayed     bool
}

// FunctionError is a function that ran and failed.
type FunctionError struct {
	InvocationID string
	Kind         functionsv1.FunctionErrorKind
	Message      string
}

func (e *FunctionError) Error() string {
	return fmt.Sprintf("function failed (%s, invocation %s): %s", e.Kind, e.InvocationID, e.Message)
}

// Invoke runs req.Function over in. A retried call with the same scope and input replays the first result.
func (c *Client) Invoke(ctx context.Context, req Request, in arrow.RecordBatch) (Result, error) {
	input, err := encodeBatch(in)
	if err != nil {
		return Result{}, fmt.Errorf("encode input: %w", err)
	}
	if len(input) > c.maxPayload {
		return Result{}, fmt.Errorf("input is %d bytes, over the %d byte limit", len(input), c.maxPayload)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	resp, err := c.fns.Invoke(ctx, connect.NewRequest(&functionsv1.InvokeRequest{
		FunctionId:     req.Function.ID,
		Version:        req.Function.Version,
		Input:          input,
		IdempotencyKey: idempotencyKey(req, input),
	}))
	if err != nil {
		return Result{}, err
	}
	msg := resp.Msg
	if fe := msg.GetError(); fe != nil {
		return Result{}, &FunctionError{InvocationID: msg.GetInvocationId(), Kind: fe.GetKind(), Message: fe.GetMessage()}
	}
	if msg.GetVersion() != req.Function.Version {
		return Result{}, fmt.Errorf("alchemy ran version %d, want %d", msg.GetVersion(), req.Function.Version)
	}
	rec, err := decodeBatch(msg.GetOutput(), c.mem)
	if err != nil {
		return Result{}, fmt.Errorf("decode output of invocation %s: %w", msg.GetInvocationId(), err)
	}
	return Result{Record: rec, InvocationID: msg.GetInvocationId(), Attempts: msg.GetAttempts(), Replayed: msg.GetReplayed()}, nil
}

// idempotencyKey hashes the scope, function and input so only an identical retry replays.
func idempotencyKey(req Request, input []byte) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00%d\x00", req.Scope, req.Function.ID, req.Function.Version)
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}
