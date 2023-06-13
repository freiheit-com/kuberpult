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
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LockServiceServer struct {
	Repository repository.Repository
}

func (l *LockServiceServer) CreateEnvironmentLock(
	ctx context.Context,
	in *api.CreateEnvironmentLockRequest) (*emptypb.Empty, error) {
	err := l.Repository.Apply(ctx, &repository.CreateEnvironmentLock{
		Environment: in.Environment,
		LockId:      in.LockId,
		Message:     in.Message,
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) DeleteEnvironmentLock(
	ctx context.Context,
	in *api.DeleteEnvironmentLockRequest) (*emptypb.Empty, error) {
	err := l.Repository.Apply(ctx, &repository.DeleteEnvironmentLock{
		Environment: in.Environment,
		LockId:      in.LockId,
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) CreateEnvironmentApplicationLock(
	ctx context.Context,
	in *api.CreateEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	err := l.Repository.Apply(ctx, &repository.CreateEnvironmentApplicationLock{
		Environment: in.Environment,
		Application: in.Application,
		LockId:      in.LockId,
		Message:     in.Message,
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (l *LockServiceServer) DeleteEnvironmentApplicationLock(
	ctx context.Context,
	in *api.DeleteEnvironmentApplicationLockRequest) (*emptypb.Empty, error) {
	err := l.Repository.Apply(ctx, &repository.DeleteEnvironmentApplicationLock{
		Environment: in.Environment,
		Application: in.Application,
		LockId:      in.LockId,
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

var _ api.LockServiceServer = (*LockServiceServer)(nil)
