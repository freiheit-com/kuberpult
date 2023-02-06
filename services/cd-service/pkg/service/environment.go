
package service

import (
	"context"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/httperrors"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type EnvironmentServiceServer struct {
	Repository repository.Repository
}

func (e *EnvironmentServiceServer) CreateEnvironment(
	ctx context.Context,
	in *api.CreateEnvironmentRequest) (*emptypb.Empty, error) {

	err := e.Repository.Apply(ctx, &repository.CreateEnvironment{
		Environment: in.Environment,
	})
	if err != nil {
		return nil, httperrors.InternalError(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
