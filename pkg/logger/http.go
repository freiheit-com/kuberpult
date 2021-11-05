package logger

import (
	"net/http"

	"go.uber.org/zap"
)

type injectLogger struct {
	logger *zap.Logger
	inner   http.Handler
}

func (i *injectLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := WithLogger(r.Context(), i.logger)
	r2  := r.Clone(ctx)
	i.inner.ServeHTTP(w, r2)
}

func WithHttpLogger(logger *zap.Logger, inner http.Handler) http.Handler {
	return &injectLogger{
		logger: logger,
		inner: inner,
	}
}
