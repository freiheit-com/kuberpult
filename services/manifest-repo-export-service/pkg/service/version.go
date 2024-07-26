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

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
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
		return nil, grpc.PublicError(ctx, fmt.Errorf("getVersion: could not find revision %v: %w", in.GitRevision, err))
	}
	state, err := o.Repository.StateAt(oid)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrorCodeNotFound {
				return nil, status.Error(codes.NotFound, "not found")
			}
		}
		return nil, err
	}
	res, err := db.WithTransactionT[api.GetVersionResponse](state.DBHandler, ctx, 1, true, func(ctx context.Context, tx *sql.Tx) (*api.GetVersionResponse, error) {
		//exhaustruct:ignore
		res := &api.GetVersionResponse{}
		version, err := state.GetEnvironmentApplicationVersion(ctx, tx, in.Environment, in.Application)
		if err != nil {
			return nil, err
		}
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
		return res, nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (o *VersionServiceServer) GetManifests(ctx context.Context, req *api.GetManifestsRequest) (*api.GetManifestsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
	// Will be implemented in Ref SRX-BVHNX1
}
