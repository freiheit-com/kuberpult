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

const (
	AlreadyExistsSame      string = "SAME CONTENT"
	AlreadyExistsDifferent        = "DIFFERENT CONTENT"
)

func InternalError(ctx context.Context, err error) error {
	logger := logger.FromContext(ctx)
	logger.Error("grpc.internal", zap.Error(err))
	return status.Error(codes.Internal, "internal error")
}

func PublicError(ctx context.Context, err error) error {
	logger := logger.FromContext(ctx)
	logger.Error("grpc.public", zap.Error(err))
	return status.Error(codes.InvalidArgument, "error: "+err.Error())
}

func CanceledError(ctx context.Context, err error) error {
	return status.Error(codes.Canceled, err.Error())
}

func AuthError(ctx context.Context, err error) error {
	return status.Error(codes.Unauthenticated, "error: "+err.Error())
}

// AlreadyExistsError from the cd service may or may not be an error in http/REST.
// If the uploaded manifest is the same as the existing manifest, this will a 200 (as opposed to a 201) -- but not an error.
func AlreadyExistsSameError(err error) error {
	return status.Error(codes.AlreadyExists, AlreadyExistsSame + ": " + err.Error())
}
// If the uploaded manifest is not the same as the existing manifest, this will be a 409 in http/REST.
func AlreadyExistsDifferentError(err error) error {
	return status.Error(codes.AlreadyExists, AlreadyExistsDifferent + ": " + err.Error())
}
