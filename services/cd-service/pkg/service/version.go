package service

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	git "github.com/libgit2/git2go/v34"
)

type VersionServiceServer struct {
	Repository repository.Repository
}

func (o *VersionServiceServer) GetVersion(
	ctx context.Context,
	in *api.GetVersionRequest) (*api.GetVersionResponse, error) {
	oid, err := git.NewOid(in.GitRevision)
	if err != nil {
		return nil, err
	}
	state, err := o.Repository.StateAt(oid)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrNotFound {
				return nil, status.Error(codes.NotFound, "not found")
			}
		}
		return nil, err
	}
	res := api.GetVersionResponse{}
	version, err := state.GetEnvironmentApplicationVersion(in.Environment, in.Application)
	if version != nil {
		res.Version = *version
		_, deployedAt, err := state.GetDeploymentMetaData(ctx, in.Environment, in.Application)
		if err != nil {
			return nil, err
		}
		res.DeployedAt = timestamppb.New(deployedAt)
	}
	return &res, nil
}
