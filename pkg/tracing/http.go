package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func WrapHttp(ctx context.Context, h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, "/", otelhttp.WithTracerProvider(ProviderFromContext(ctx)))
}
