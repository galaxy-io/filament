package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/alchemy"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/transform"
)

// maxFunctionError caps the function-supplied text a failure fact carries.
const maxFunctionError = 1 << 10

// Functions invokes alchemy functions; *alchemy.Client satisfies it.
type Functions interface {
	Invoke(ctx context.Context, req alchemy.Request, in arrow.RecordBatch) (alchemy.Result, error)
}

// invoker binds alchemy steps for resource to the pipeline's client, or nil when it has none.
func (p *Pipeline) invoker(resource string) transform.Invoker {
	if p.functions == nil {
		return nil
	}
	return functionInvoker{p: p, resource: resource}
}

// functionInvoker scopes idempotency to the run and step, and reports every call as a fact.
type functionInvoker struct {
	p        *Pipeline
	resource string
}

func (i functionInvoker) Invoke(ctx context.Context, call transform.Call, in arrow.RecordBatch) (arrow.RecordBatch, error) {
	started := time.Now()
	scope := fmt.Sprintf("%s/%s/%s/%s", i.p.tenant, i.p.run, i.resource, call.Step)
	res, err := i.p.functions.Invoke(alchemy.WithTenant(ctx, i.p.tenant), alchemy.Request{Function: call.Function, Scope: scope}, in)
	env := events.Envelope{Resource: i.resource}
	if err != nil {
		failed := events.FunctionFailedEvent{
			Function: call.Function.ID,
			Version:  call.Function.Version,
			Step:     call.Step,
			Code:     connect.CodeOf(err).String(),
			Error:    truncate(err.Error(), maxFunctionError),
			Duration: time.Since(started),
		}
		var fe *alchemy.FunctionError
		if errors.As(err, &fe) {
			failed.InvocationID = fe.InvocationID
			failed.Code = fe.Kind.String()
		}
		i.p.publish(events.NewFact(events.FunctionFailed, env, failed))
		return nil, err
	}
	i.p.publish(events.NewFact(events.FunctionInvoked, env, events.FunctionInvokedEvent{
		Function:     call.Function.ID,
		Version:      call.Function.Version,
		Step:         call.Step,
		InvocationID: res.InvocationID,
		Records:      res.Record.NumRows(),
		Attempts:     res.Attempts,
		Replayed:     res.Replayed,
		Duration:     time.Since(started),
	}))
	return res.Record, nil
}

// truncate cuts s to at most n bytes on a rune boundary.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}
