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
	"github.com/freiheit-com/kuberpult/pkg/types"
	"os"

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
	envName := types.EnvName(in.Environment)
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
		version, err := state.GetEnvironmentApplicationVersion(ctx, tx, envName, in.Application)
		if err != nil {
			return nil, err
		}
		if version.Version != nil {
			res.Version = *version.Version
			_, deployedAt, err := state.GetDeploymentMetaData(envName, in.Application)
			if err != nil {
				return nil, err
			}
			res.DeployedAt = timestamppb.New(deployedAt)
			release, err := state.GetApplicationRelease(in.Application, version)
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
	if req.Application == "" {
		return nil, status.Error(codes.InvalidArgument, "no application specified")
	}

	state := o.Repository.State()

	wrapError := func(what string, err error) error {
		if !os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, "%s not found", what)
		} else {
			return status.Error(codes.Internal, err.Error())
		}
	}

	result, err := db.WithTransactionT(state.DBHandler, ctx, 1, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetManifestsResponse, error) {
		var (
			err     error
			release types.ReleaseNumbers
		)
		if req.Release == "latest" {
			release, err = state.GetLastRelease(ctx, state.Filesystem, req.Application)
			if err != nil {
				return nil, wrapError("application", err)
			}
			if release.Version == nil {
				return nil, status.Errorf(codes.NotFound, "no releases found for application %s", req.Application)
			}
		} else {
			release, err = types.MakeReleaseNumberFromString(req.Release)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid release number, expected number, 'Major.Minor' or 'latest'")
			}
		}
		repoRelease, err := state.GetApplicationRelease(req.Application, release)
		if err != nil {
			return nil, wrapError("release", err)
		}
		manifests, err := state.GetApplicationReleaseManifests(req.Application, release)
		if err != nil {
			return nil, wrapError("manifests", err)
		}

		return &api.GetManifestsResponse{
			Release:   repoRelease.ToProto(),
			Manifests: manifests,
		}, nil
	})
	return result, err
}
