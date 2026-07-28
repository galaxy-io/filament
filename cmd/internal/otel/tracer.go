package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/galaxy-io/filament"
)

type tracer struct{ t trace.Tracer }

var _ filament.Tracer = tracer{}

func (t tracer) Start(ctx context.Context, name string) (context.Context, filament.Span) {
	ctx, s := t.t.Start(ctx, name)
	return ctx, span{s: s}
}

type span struct{ s trace.Span }

func (s span) End() { s.s.End() }

func (s span) SetError(err error) {
	if err == nil {
		return
	}
	s.s.RecordError(err)
	s.s.SetStatus(codes.Error, err.Error())
}

func (s span) SetAttr(key string, v any) {
	switch v := v.(type) {
	case string:
		s.s.SetAttributes(attribute.String(key, v))
	case bool:
		s.s.SetAttributes(attribute.Bool(key, v))
	case int:
		s.s.SetAttributes(attribute.Int(key, v))
	case int64:
		s.s.SetAttributes(attribute.Int64(key, v))
	case float64:
		s.s.SetAttributes(attribute.Float64(key, v))
	default:
		s.s.SetAttributes(attribute.String(key, fmt.Sprint(v)))
	}
}
