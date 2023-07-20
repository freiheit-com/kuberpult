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
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/httperrors"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DeployServiceServer struct {
	Repository repository.Repository
}

func (d *DeployServiceServer) Deploy(
	ctx context.Context,
	in *api.DeployRequest,
) (*emptypb.Empty, error) {
	if err := ValidateDeployment(in.Environment, in.Application); err != nil {
		return nil, err
	}
	b := in.LockBehavior
	if in.IgnoreAllLocks {
		// the UI currently sets this to true,
		// in that case, we still want to ignore locks (for emergency deployments)
		b = api.LockBehavior_Ignore
	}
	err := d.Repository.Apply(ctx, &repository.DeployApplicationVersion{
		Environment:   in.Environment,
		Application:   in.Application,
		Version:       in.Version,
		LockBehaviour: b,
	})

	if err != nil {
		var lockedErr *repository.LockedError
		if errors.As(err, &lockedErr) {
			detail := &api.LockedError{
				EnvironmentLocks:            map[string]*api.Lock{},
				EnvironmentApplicationLocks: map[string]*api.Lock{},
			}
			for k, v := range lockedErr.EnvironmentLocks {
				detail.EnvironmentLocks[k] = &api.Lock{
					Message: v.Message,
				}
			}
			for k, v := range lockedErr.EnvironmentApplicationLocks {
				detail.EnvironmentApplicationLocks[k] = &api.Lock{
					Message: v.Message,
				}
			}
			stat, sErr := status.New(codes.FailedPrecondition, "locked").WithDetails(detail)
			if sErr != nil {
				return nil, httperrors.InternalError(ctx, sErr)
			}
			return nil, stat.Err()
		}
		return nil, httperrors.InternalError(ctx, err)
	}

	return &emptypb.Empty{}, nil
}

func (d *DeployServiceServer) ReleaseTrain(
	ctx context.Context,
	in *api.ReleaseTrainRequest,
) (*api.ReleaseTrainResponse, error) {
	return nil, nil
}
