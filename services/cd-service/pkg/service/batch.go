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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BatchServer struct {
	Repository repository.Repository
}

func (d *BatchServer) processAction(
	batchAction *api.BatchAction,
) (repository.Transformer , error) {
	switch action := batchAction.Action.(type) {
	case *api.BatchAction_CreateEnvironmentLock:
		if !valid.EnvironmentName(action.CreateEnvironmentLock.Environment) {
			return nil, status.Error(codes.InvalidArgument, "invalid environment")
		}
		if !valid.LockId(action.CreateEnvironmentLock.LockId) {
			return nil, status.Error(codes.InvalidArgument, "invalid lock id")
		}
		return &repository.CreateEnvironmentLock{
			Environment: action.CreateEnvironmentLock.Environment,
			LockId:      action.CreateEnvironmentLock.LockId,
			Message:     action.CreateEnvironmentLock.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentLock:
		if !valid.EnvironmentName(action.DeleteEnvironmentLock.Environment) {
			return nil, status.Error(codes.InvalidArgument, "invalid environment")
		}
		if !valid.LockId(action.DeleteEnvironmentLock.LockId) {
			return nil, status.Error(codes.InvalidArgument, "invalid lock id")
		}
		return &repository.DeleteEnvironmentLock{
			Environment: action.DeleteEnvironmentLock.Environment,
			LockId:      action.DeleteEnvironmentLock.LockId,
		}, nil
	case *api.BatchAction_CreateEnvironmentApplicationLock:
		if !valid.EnvironmentName(action.CreateEnvironmentApplicationLock.Environment) {
			return nil, status.Error(codes.InvalidArgument, "invalid environment")
		}
		if !valid.ApplicationName(action.CreateEnvironmentApplicationLock.Application) {
			return nil, status.Error(codes.InvalidArgument, "invalid application")
		}
		if !valid.LockId(action.CreateEnvironmentApplicationLock.LockId) {
			return nil, status.Error(codes.InvalidArgument, "invalid lock id")
		}
		return &repository.CreateEnvironmentApplicationLock{
			Environment: action.CreateEnvironmentApplicationLock.Environment,
			Application: action.CreateEnvironmentApplicationLock.Application,
			LockId:      action.CreateEnvironmentApplicationLock.LockId,
			Message:     action.CreateEnvironmentApplicationLock.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentApplicationLock:
		if !valid.EnvironmentName(action.DeleteEnvironmentApplicationLock.Environment) {
			return nil, status.Error(codes.InvalidArgument, "invalid environment")
		}
		if !valid.ApplicationName(action.DeleteEnvironmentApplicationLock.Application) {
			return nil, status.Error(codes.InvalidArgument, "invalid application")
		}
		if !valid.LockId(action.DeleteEnvironmentApplicationLock.LockId) {
			return nil, status.Error(codes.InvalidArgument, "invalid lock id")
		}
		return &repository.DeleteEnvironmentApplicationLock{
			Environment: action.DeleteEnvironmentApplicationLock.Environment,
			Application: action.DeleteEnvironmentApplicationLock.Application,
			LockId:      action.DeleteEnvironmentApplicationLock.LockId,
		}, nil
	case *api.BatchAction_Deploy:
		if !valid.EnvironmentName(action.Deploy.Environment) {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid environment %q", action.Deploy.Environment))
		}
		if !valid.ApplicationName(action.Deploy.Application) {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid application %q", action.Deploy.Application))
		}
		b := action.Deploy.LockBehavior
		if action.Deploy.IgnoreAllLocks {
			// the UI currently sets this to true,
			// in that case, we still want to ignore locks (for emergency deployments)
			b = api.LockBehavior_Ignore
		}
		return &repository.DeployApplicationVersion{
			Environment:   action.Deploy.Environment,
			Application:   action.Deploy.Application,
			Version:       action.Deploy.Version,
			LockBehaviour: b,
		}, nil
	}
	return nil, status.Error(codes.InvalidArgument, "invalid action")
}

func (d *BatchServer) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest,
) (*emptypb.Empty, error) {
	if len(in.GetActions())>100 {
		return nil, status.Error(codes.InvalidArgument, "too many actions. limit is 100")
	}
	transformers := make ([]repository.Transformer, 0, 100)
	for _, batchAction := range in.GetActions() {
		transformer, err := d.processAction(batchAction)
		if err != nil {
			// Validation error
			return nil, err
		}
		transformers = append(transformers, transformer)
	}
	err := d.Repository.Apply(ctx, transformers...)
	if err != nil {
		// TODO TE: error handling
		return nil, internalError(ctx, err)
	}
	return &emptypb.Empty{}, nil
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
