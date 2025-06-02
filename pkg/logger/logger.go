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

// Log implementation for all microservices in the project.
// Log functions can be called through the convenience interfaces
// logger.Debugf(), logger.Errorf(), logger.Panicf()
//
// Deliberately reduces the interface to only Debugf, Errorf and Panicf.
// The other log levels are discouraged (see fdc Software Engineering Standards
// for details)
package logger

import (
	"context"
	"fmt"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"os"

	"github.com/blendle/zapdriver"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func FromContext(ctx context.Context) *zap.Logger {
	l := ctxzap.Extract(ctx)
	span, ok := tracer.SpanFromContext(ctx)
	if ok {
		env := os.Getenv("DD_ENV")
		service := os.Getenv("DD_SERVICE")
		version := os.Getenv("DD_VERSION")
		return l.With(
			zap.Uint64("dd.trace_id", span.Context().TraceID()),
			zap.Uint64("dd.span_id", span.Context().SpanID()),
			zap.String("dd.env", env),
			zap.String("dd.service", service),
			zap.String("dd.version", version),
		)
	}
	return l
}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return ctxzap.ToContext(ctx, logger)
}

func Wrap(ctx context.Context, inner func(ctx context.Context) error) error {
	format := os.Getenv("LOG_FORMAT")
	envLevel := os.Getenv("LOG_LEVEL")
	var (
		logger *zap.Logger
		level  zapcore.Level = zapcore.WarnLevel
		err    error
	)
	if envLevel != "" {
		err = level.Set(envLevel)
		if err != nil {
			return err
		}
	}
	options := []zap.Option{zap.IncreaseLevel(level)}
	switch format {
	case "gcp":
		logger, err = zapdriver.NewProduction(options...)
	case "", "default":
		logger, err = zap.NewProduction(options...)
	default:
		return fmt.Errorf("unknown log_format: %s", format)
	}
	if err != nil {
		return err
	}
	defer func() {
		syncErr := logger.Sync()
		if err == nil {
			err = syncErr
		}
	}()
	err = inner(WithLogger(ctx, logger))

	return err
}
