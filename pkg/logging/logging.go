/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

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

func HandlePanic(exitOnPanic bool) {
	logger.HandlePanic(exitOnPanic)
}
