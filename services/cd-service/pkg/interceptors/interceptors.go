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

package interceptors

import (
	"context"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/auth"
	"google.golang.org/grpc"
)

// UnaryUserContextInterceptor assumes that there is a user in the context
// if there is no user, it will return an auth error.
// if there is a user, it will be written to the context.
func UnaryUserContextInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
	reader auth.GrpcContextReader) (interface{}, error) {

	if strings.HasPrefix(info.FullMethod, "/repository.RepoServerService/") {
		return handler(ctx, req)
	}
	user, err := reader.ReadUserFromGrpcContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx = auth.WriteUserToContext(ctx, *user)
	ctx = context.WithoutCancel(ctx)
	h, err := handler(ctx, req)
	return h, err
}
