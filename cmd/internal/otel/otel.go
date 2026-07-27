// Package otel selects the observability providers: OTLP-backed when an
// OTLP endpoint is configured, noop otherwise.
package otel

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/galaxy-io/filament"
)

const scope = "github.com/galaxy-io/filament"

// FromEnv builds Metrics and Tracer providers. When no OTEL_EXPORTER_OTLP_*
// endpoint is set they wrap otel's built-in noops. Otherwise they export OTLP
// over gRPC; the exporters and resource read the standard OTEL_* variables
// (endpoint, headers, service name, sampler). Call shutdown on exit to flush.
func FromEnv(ctx context.Context) (filament.Metrics, filament.Tracer, func(context.Context) error, error) {
	if !enabled() {
		return metrics{m: noopmetric.NewMeterProvider().Meter(scope)},
			tracer{t: nooptrace.NewTracerProvider().Tracer(scope)},
			func(context.Context) error { return nil }, nil
	}
	me, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("otel: metric exporter: %w", err)
	}
	te, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("otel: trace exporter: %w", err)
	}
	res := resource.Default()
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me)),
	)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(te),
	)
	// Register globally so otel-instrumented libraries (otelhttp, otelgrpc)
	// export through the same providers.
	otel.SetMeterProvider(mp)
	otel.SetTracerProvider(tp)
	shutdown := func(ctx context.Context) error {
		return errors.Join(mp.Shutdown(ctx), tp.Shutdown(ctx))
	}
	return metrics{m: mp.Meter(scope)}, tracer{t: tp.Tracer(scope)}, shutdown, nil
}

func enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}
