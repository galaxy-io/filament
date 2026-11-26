// Package otel selects the observability providers: OTLP-backed for each
// signal an endpoint is configured for, noop otherwise.
package otel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/telemetry"
	"github.com/galaxy-io/filament/version"
)

const (
	scope = "github.com/galaxy-io/filament"

	sdkDisabledEnv     = "OTEL_SDK_DISABLED"
	endpointEnv        = "OTEL_EXPORTER_OTLP_ENDPOINT"
	metricsEndpointEnv = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	tracesEndpointEnv  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	protocolEnv        = "OTEL_EXPORTER_OTLP_PROTOCOL"
	metricsProtocolEnv = "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL"
	tracesProtocolEnv  = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
	metricsExporterEnv = "OTEL_METRICS_EXPORTER"
	tracesExporterEnv  = "OTEL_TRACES_EXPORTER"

	exporterNone = "none"
	protocolGRPC = "grpc"
	protocolHTTP = "http/protobuf"
)

// FromEnv builds the metrics Set and the Tracer. Each signal exports over OTLP
// when OTEL_EXPORTER_OTLP_ENDPOINT or its own endpoint variable is set, unless
// OTEL_METRICS_EXPORTER or OTEL_TRACES_EXPORTER is none, so an endpoint that
// takes only metrics is sent no spans. OTEL_SDK_DISABLED=true turns every
// signal off. A signal that is off records nothing. Each signal's transport is
// its OTEL_EXPORTER_OTLP_{METRICS,TRACES}_PROTOCOL, else
// OTEL_EXPORTER_OTLP_PROTOCOL, else gRPC. The exporters and resource read the
// other standard OTEL_* variables (headers, timeout, compression, service
// name, resource attributes, sampler). SDK and export errors are logged to log
// at warn level. Call shutdown on exit to flush.
func FromEnv(ctx context.Context, log filament.Logger) (*telemetry.Set, filament.Tracer, func(context.Context) error, error) {
	var metrics *telemetry.Set
	tr := tracer{t: nooptrace.NewTracerProvider().Tracer(scope)}
	var shutdowns []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		errs := make([]error, 0, len(shutdowns))
		for _, stop := range shutdowns {
			errs = append(errs, stop(ctx))
		}
		return errors.Join(errs...)
	}
	if sdkDisabled() {
		return nil, tr, shutdown, nil
	}
	if log != nil {
		otel.SetErrorHandler(errorHandler(log))
	}
	res := processResource()

	if exports(metricsExporterEnv, metricsEndpointEnv) {
		exporter, err := metricExporter(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		provider := sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		)
		// Registered globally so otel-instrumented libraries (otelhttp,
		// otelgrpc) export through the same provider.
		otel.SetMeterProvider(provider)
		metrics = telemetry.New(backend{m: provider.Meter(scope, metric.WithInstrumentationVersion(version.Version))})
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	if exports(tracesExporterEnv, tracesEndpointEnv) {
		exporter, err := traceExporter(ctx)
		if err != nil {
			return nil, nil, nil, errors.Join(err, shutdown(ctx))
		}
		provider := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(exporter),
		)
		otel.SetTracerProvider(provider)
		tr = tracer{t: provider.Tracer(scope, trace.WithInstrumentationVersion(version.Version))}
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	return metrics, tr, shutdown, nil
}

func sdkDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(sdkDisabledEnv)), "true")
}

func errorHandler(log filament.Logger) otel.ErrorHandler {
	return otel.ErrorHandlerFunc(func(err error) {
		log.Warn("opentelemetry error",
			filament.Field{Key: "event.name", Value: "otel.error"},
			filament.Field{Key: "error", Value: err.Error()})
	})
}

// exports reports whether a signal is sent: it has an endpoint, shared or its
// own, and its exporter variable does not turn it off.
func exports(exporterEnv, signalEndpointEnv string) bool {
	if os.Getenv(exporterEnv) == exporterNone {
		return false
	}
	return os.Getenv(endpointEnv) != "" || os.Getenv(signalEndpointEnv) != ""
}

func protocol(signalProtocolEnv string) (env, value string) {
	for _, env := range []string{signalProtocolEnv, protocolEnv} {
		if value := os.Getenv(env); value != "" {
			return env, value
		}
	}
	return protocolEnv, protocolGRPC
}

func metricExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	var exporter sdkmetric.Exporter
	var err error
	switch env, value := protocol(metricsProtocolEnv); value {
	case protocolGRPC:
		exporter, err = otlpmetricgrpc.New(ctx)
	case protocolHTTP:
		exporter, err = otlpmetrichttp.New(ctx)
	default:
		return nil, unknownProtocol(env, value)
	}
	if err != nil {
		return nil, fmt.Errorf("otel: metric exporter: %w", err)
	}
	return exporter, nil
}

func traceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	var exporter sdktrace.SpanExporter
	var err error
	switch env, value := protocol(tracesProtocolEnv); value {
	case protocolGRPC:
		exporter, err = otlptracegrpc.New(ctx)
	case protocolHTTP:
		exporter, err = otlptracehttp.New(ctx)
	default:
		return nil, unknownProtocol(env, value)
	}
	if err != nil {
		return nil, fmt.Errorf("otel: trace exporter: %w", err)
	}
	return exporter, nil
}

func unknownProtocol(env, value string) error {
	return fmt.Errorf("otel: unknown %s %q (%s, %s)", env, value, protocolGRPC, protocolHTTP)
}

// processResource is the default resource with the build version as
// service.version and the hostname as service.instance.id, so replicas of one
// service export separate series. Values in OTEL_RESOURCE_ATTRIBUTES and
// OTEL_SERVICE_NAME win.
func processResource() *resource.Resource {
	attrs := []attribute.KeyValue{semconv.ServiceVersion(version.Version)}
	if hostname, err := os.Hostname(); err == nil {
		attrs = append(attrs, semconv.ServiceInstanceID(hostname))
	}
	merged, err := resource.Merge(resource.NewSchemaless(attrs...), resource.Default())
	if err != nil {
		return resource.Default()
	}
	return merged
}
