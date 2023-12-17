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
		release, err := state.GetApplicationRelease(in.Application, *version)
		if err != nil {
			return nil, err
		}
		res.SourceCommitId = release.SourceCommitId
	}
	return &res, nil
}
