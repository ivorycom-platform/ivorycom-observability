// Package obs is Ivorycom's shared OpenTelemetry bootstrap for Go services.
//
// One call — Init — wires traces, metrics, and logs to the Grafana Alloy
// collector over OTLP/HTTP. Services then use Logger, FiberMiddleware /
// REDMiddleware, and HTTPClient without touching OTel internals directly.
//
// House style: Fiber (fasthttp) + zerolog. The OTLP endpoint is read from
// OTEL_EXPORTER_OTLP_ENDPOINT (e.g. http://observability-collector.railway.internal:4318).
package obs

import (
	"context"
	"errors"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config identifies the service to the telemetry backend.
type Config struct {
	// Service is the OTel service.name (e.g. "auth").
	Service string
	// Env is the deployment environment (e.g. "prod").
	Env string
}

// loggerProvider is stashed at Init so Logger(ctx) can emit to the OTLP
// LoggerProvider. nil until Init runs (Logger degrades to stdout-only).
var loggerProvider *sdklog.LoggerProvider

// Init sets global tracer/meter/logger providers exporting OTLP/HTTP to the
// endpoint in OTEL_EXPORTER_OTLP_ENDPOINT and installs the W3C propagator.
// The returned shutdown flushes and stops all three providers.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return nil, errors.New("obs: OTEL_EXPORTER_OTLP_ENDPOINT is required")
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", cfg.Service),
		attribute.String("deployment.environment.name", cfg.Env),
	))
	if err != nil {
		return nil, err
	}

	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)

	logExp, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	loggerProvider = lp

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}
