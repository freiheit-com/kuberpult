package logging

import (
	"context"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
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