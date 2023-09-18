package tracing

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func OTELUnaryClientInterceptor(ctx context.Context) grpc.UnaryClientInterceptor {
	return otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(ProviderFromContext(ctx)))
}

func OTELStreamClientInterceptor(ctx context.Context) grpc.StreamClientInterceptor {
	return otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(ProviderFromContext(ctx)))
}

func OTELUnaryServerInterceptor(ctx context.Context) grpc.UnaryServerInterceptor {
	return otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(ProviderFromContext(ctx)))
}

func OTELStreamServerInterceptor(ctx context.Context) grpc.StreamServerInterceptor {
	return otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(ProviderFromContext(ctx)))
}
