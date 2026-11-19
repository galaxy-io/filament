package pipeline

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/transform"
)

// maxFunctionError caps the function-supplied text a failure fact carries.
const maxFunctionError = 1 << 10

// invoker binds alchemy steps for resource to the pipeline's function service, or nil when it has none.
func (p *Pipeline) invoker(resource string) transform.Invoker {
	if p.functions == nil {
		return nil
	}
	return functionInvoker{p: p, resource: resource}
}

// functionInvoker runs calls as the run's tenant and reports every remote invocation as a fact.
type functionInvoker struct {
	p        *Pipeline
	resource string
}

func (i functionInvoker) Invoke(ctx context.Context, call transform.Call, in arrow.RecordBatch) (arrow.RecordBatch, error) {
	started := time.Now()
	res, err := i.p.functions.Invoke(ctx, i.p.tenant, call, in)
	env := events.Envelope{Resource: i.resource}
	for _, inv := range res.Invocations {
		i.p.publish(events.NewFact(events.FunctionInvoked, env, events.FunctionInvokedEvent{
			Function:     call.Function.ID,
			Version:      call.Function.Version,
			Step:         call.Step,
			InvocationID: inv.ID,
			Records:      inv.Records,
			Attempts:     inv.Attempts,
			Replayed:     inv.Replayed,
			Duration:     inv.Duration,
		}))
	}
	if err != nil {
		failed := events.FunctionFailedEvent{
			Function: call.Function.ID,
			Version:  call.Function.Version,
			Step:     call.Step,
			Code:     connect.CodeOf(err).String(),
			Error:    truncate(err.Error(), maxFunctionError),
			Duration: time.Since(started),
		}
		var fe *transform.FunctionError
		if errors.As(err, &fe) {
			failed.InvocationID = fe.InvocationID
			failed.Code = fe.Kind
		}
		i.p.publish(events.NewFact(events.FunctionFailed, env, failed))
		return nil, err
	}
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
