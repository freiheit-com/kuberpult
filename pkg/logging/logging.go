package logging

import (
	"context"

	"go.uber.org/zap"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

func Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	logger.FromContext(ctx).Fatal(msg, fields...)
}

func Error(ctx context.Context, msg string, fields ...zap.Field) {
	logger.FromContext(ctx).Error(msg, fields...)
}

func Warn(ctx context.Context, msg string, fields ...zap.Field) {
	logger.FromContext(ctx).Warn(msg, fields...)
}

func Info(ctx context.Context, msg string, fields ...zap.Field) {
	logger.FromContext(ctx).Info(msg, fields...)
}
