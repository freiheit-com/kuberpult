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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
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
		// Note that "not finding a oid" does not mean that it doesn't exist.
		// Because we do a shallow clone, we won't have information on all existing OIDs.
		return nil, grpc.PublicError(ctx, fmt.Errorf("getVersion: could not find revision %v: %v", in.GitRevision, err))
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
	//exhaustruct:ignore
	res := api.GetVersionResponse{}
	version, err := state.GetEnvironmentApplicationVersion(in.Environment, in.Application)
	if version != nil {
		res.Version = *version
		_, deployedAt, err := state.GetDeploymentMetaData(in.Environment, in.Application)
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
