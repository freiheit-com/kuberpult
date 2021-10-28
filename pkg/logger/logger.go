/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
//
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
	"os"

	"go.uber.org/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"github.com/blendle/zapdriver"
)

func FromContext(ctx context.Context) *zap.Logger {
	return ctxzap.Extract(ctx)
}

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return ctxzap.ToContext(ctx, logger)
}

func Start(ctx context.Context) context.Context {
	format := os.Getenv("LOG_FORMAT")
	var (
		logger *zap.Logger
		err error
	)
	switch format {
	case "gcp":
		logger, err = zapdriver.NewProduction()
	default:
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	return WithLogger(ctx, logger)
}
