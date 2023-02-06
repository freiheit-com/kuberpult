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
package httperrors

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

func PublicError(ctx context.Context, err error) error {
	logger := logger.FromContext(ctx)
	logger.Error("grpc.internal", zap.Error(err))
	return status.Error(codes.InvalidArgument, "error: " + err.Error())
}
