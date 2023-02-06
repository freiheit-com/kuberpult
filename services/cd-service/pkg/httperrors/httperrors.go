
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
