
package handler

import (
	"context"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func handleGRPCError(ctx context.Context, w http.ResponseWriter, err error) {
	s, _ := status.FromError(err)
	switch s.Code() {
	case codes.InvalidArgument:
		http.Error(w, s.Message(), http.StatusBadRequest)
	default:
		logger.FromContext(ctx).Error(s.Message())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
