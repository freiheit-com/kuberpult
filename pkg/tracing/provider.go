package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.16.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

var otlpAddress = "jaeger-all-in-one:4317"

func Provider(ctx context.Context, serviceName string) (oteltrace.TracerProvider, context.CancelFunc, error) {

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
		//resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up a trace exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(otlpAddress),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return provider, func() {
		exporter.Shutdown(ctx)
	}, nil
}

type ctxKey struct{}

var key ctxKey = ctxKey{}

func WithTracerProvider(ctx context.Context, serviceName string) (context.Context, context.CancelFunc, error) {
	tp, fn, err := Provider(ctx, serviceName)
	if err != nil {
		return nil, fn, err
	}
	return context.WithValue(ctx, key, tp), fn, nil
}

func ProviderFromContext(ctx context.Context) oteltrace.TracerProvider {
	tp := ctx.Value(key)
	if tp == nil {
		panic("no tracer provider in ctx")
	}
	return tp.(oteltrace.TracerProvider)
}


func Start(ctx context.Context, spanName string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
  return ProviderFromContext(ctx).Tracer("kuberpult").Start(ctx, spanName, opts...)
}
