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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"os"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type VersionServiceServer struct {
	Repository repository.Repository
}

func (o *VersionServiceServer) GetVersion(
	ctx context.Context,
	in *api.GetVersionRequest) (*api.GetVersionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
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

	result, err := db.WithTransactionT(state.DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetManifestsResponse, error) {
		var (
			err     error
			release uint64
		)

		if req.Release == "latest" {
			release, err = state.GetLastRelease(ctx, transaction, req.Application)
			if err != nil {
				return nil, wrapError("application", err)
			}
			if release == 0 {
				return nil, status.Errorf(codes.NotFound, "no releases found for application %s", req.Application)
			}
		} else {
			release, err = strconv.ParseUint(req.Release, 10, 64)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid release number, expected uint or 'latest'")
			}
		}
		repoRelease, err := state.GetApplicationRelease(ctx, transaction, req.Application, release)
		if err != nil {
			return nil, wrapError("release", err)
		}
		manifests, err := state.GetApplicationReleaseManifests(ctx, transaction, req.Application, release)
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
