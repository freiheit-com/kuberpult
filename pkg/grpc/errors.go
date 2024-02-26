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

Copyright 2023 freiheit.com*/

package grpc

import (
	"context"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func InternalError(ctx context.Context, err error) error {
	logger := logger.FromContext(ctx)
	logger.Error("grpc.internal", zap.Error(err))
	return status.Error(codes.Internal, "internal error")
}

func PublicError(_ context.Context, err error) error {
	return status.Error(codes.InvalidArgument, "error: "+err.Error())
}

func CanceledError(_ context.Context, err error) error {
	return status.Error(codes.Canceled, err.Error())
}

func FailedPrecondition(_ context.Context, err error) error {
	return status.Error(codes.FailedPrecondition, "error: "+err.Error())
}

func AuthError(_ context.Context, err error) error {
	return status.Error(codes.Unauthenticated, "error: "+err.Error())
}

func NotFoundError(_ context.Context, err error) error {
	return status.Error(codes.NotFound, "error: "+err.Error())
}
