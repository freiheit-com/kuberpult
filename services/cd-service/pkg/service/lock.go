/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package service

import (
	"context"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"strings"
)

type LockServiceServer struct {
	Repository        repository.Repository
	HealthCheckResult HealthCheckResultPtr
}

func (l *LockServiceServer) CreateEnvironmentLock(
	ctx context.Context,
	in *api.CreateEnvironmentLockRequest) (*emptypb.Empty, error) {
	err := ValidateEnvironmentLock("create", in.Environment, in.LockId)
	if err != nil {
		return nil, err
	}
	err = l.Repository.Apply(ctx, &repository.CreateEnvironmentLock{
		Environment: in.Environment,
		LockId:      in.LockId,
		Message:     in.Message,
	})
	if err != nil {
		log := logger.FromContext(ctx)
		log.Error("Internal Error in Create env lock!", zap.Error(err))
		return nil, internalError(ctx, err, l.HealthCheckResult)
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) DeleteEnvironmentLock(
	ctx context.Context,
	in *api.DeleteEnvironmentLockRequest) (*emptypb.Empty, error) {
	err := ValidateEnvironmentLock("delete", in.Environment, in.LockId)
	if err != nil {
		return nil, err
	}
	err = l.Repository.Apply(ctx, &repository.DeleteEnvironmentLock{
		Environment: in.Environment,
		LockId:      in.LockId,
	})
	if err != nil {
		return nil, internalError(ctx, err, l.HealthCheckResult)
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) CreateEnvironmentApplicationLock(
	ctx context.Context,
	in *api.CreateEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	err := ValidateEnvironmentApplicationLock("create", in.Environment, in.Application, in.LockId)
	if err != nil {
		return nil, err
	}
	err = l.Repository.Apply(ctx, &repository.CreateEnvironmentApplicationLock{
		Environment: in.Environment,
		Application: in.Application,
		LockId:      in.LockId,
		Message:     in.Message,
	})
	if err != nil {
		return nil, internalError(ctx, err, l.HealthCheckResult)
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) DeleteEnvironmentApplicationLock(
	ctx context.Context,
	in *api.DeleteEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	log := logger.FromContext(ctx)
	log.Error("DeleteEnvironmentApplicationLock 1", zap.Error(nil))
	err := ValidateEnvironmentApplicationLock("delete", in.Environment, in.Application, in.LockId)
	if err != nil {
		return nil, err
	}
	log.Error("DeleteEnvironmentApplicationLock 2", zap.Error(nil))
	err = l.Repository.Apply(ctx, &repository.DeleteEnvironmentApplicationLock{
		Environment: in.Environment,
		Application: in.Application,
		LockId:      in.LockId,
	})
	log.Error("DeleteEnvironmentApplicationLock 3", zap.Error(nil))
	if err != nil {
		return nil, internalError(ctx, err, l.HealthCheckResult)
	}
	log.Error("DeleteEnvironmentApplicationLock 4", zap.Error(nil))

	return &emptypb.Empty{}, nil
}

func internalError(ctx context.Context, err error, result *HealthCheckResult) error {
	if strings.Contains(strings.ToLower(err.Error()), "no space left on device") {
		// detected that we ran out of storage
		// we can't do anything here, except restart the pod (to get a new storage)
		result.OK = false
		result.HttpCode =507
	}
	return status.Error(codes.Internal, "internal error")
}

var _ api.LockServiceServer = (*LockServiceServer)(nil)
