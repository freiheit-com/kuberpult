package logging

import (
	"context"

	"go.uber.org/zap"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func fromContext(ctx context.Context) *zap.Logger {
	// serves as a proxy for logger.FromContext(ctx)
	return logger.FromContext(ctx)
}

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	fromContext(ctx).Fatal(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	fromContext(ctx).Error(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	fromContext(ctx).Warn(msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	fromContext(ctx).Info(msg, fields...)
}
