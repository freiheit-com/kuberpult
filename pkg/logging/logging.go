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

// ApiDeprecationWarning should be called of the start of each endpoint that is deprecated and will be replaced by a new endpoint in /api/
// The "oldEndpoint" param should be the currently used URI.
// The "newEndpoint" param is the corresponding endpoint in the new api (/api/...)
// The "method" param is assumed to be the same for both new and old
func ApiDeprecationWarning(ctx context.Context, oldEndpoint string, newEndpoint string, method string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("oldEndpoint", oldEndpoint),
		zap.String("newEndpoint", newEndpoint),
		zap.String("method", method),
		zap.String("notes", "the old endpoint is deprecated, it is recommended to use the new endpoint"),
	}
	allFields = append(allFields, fields...)
	Warn(ctx, "api deprecation with replacement", allFields...)
}

// ApiDeprecationWarningWithoutReplacement should be called for endpoints that have no replacement in /api/
func ApiDeprecationWarningWithoutReplacement(ctx context.Context, oldEndpoint string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("oldEndpoint", oldEndpoint),
	}
	allFields = append(allFields, fields...)
	Warn(ctx, "api deprecation without replacement", allFields...)
}

func Wrap(ctx context.Context, inner func(ctx context.Context) error) {
	err := logger.Wrap(ctx, inner)
	if err != nil {
	    Error(ctx, "wrap", zap.Error(err));
	}
}
