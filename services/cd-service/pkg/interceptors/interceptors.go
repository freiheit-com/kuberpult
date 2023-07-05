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

package interceptors

import (
	"context"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc"
)

// UnaryUserContextInterceptor assumes that there is a user in the context
// if there is no user, it will return an auth error.
// if there is a user, it will be written to the context.
func UnaryUserContextInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	logger.FromContext(ctx).Warn(fmt.Sprintf("UnaryUserContextInterceptor start "))

	user, err := auth.ReadUserFromGrpcContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx = auth.WriteUserToContext(ctx, *user)

	h, err := handler(ctx, req)
	return h, err
}
