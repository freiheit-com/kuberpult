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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BatchServer struct {
	Repository repository.Repository
}

func (d *BatchServer) validateAction (
	ctx context.Context,
	batchAction *api.BatchAction,
) (*emptypb.Empty, error) {
	switch action := batchAction.(type) {
	case *api.BatchAction_CreateEnvironmentLock:

	case *api.BatchAction_DeleteEnvironmentLock:

	case *api.BatchAction_CreateApplicationLock:

	case *api.BatchAction_DeleteApplicationLock:

	case *api.BatchAction_Deploy:

	}
	return &emptypb.Empty{}, nil
}

func (d *BatchServer) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest,
) (*emptypb.Empty, error) {
	if len(in.GetActions())>100 {
		return nil, status.Error(codes.InvalidArgument, "too many actions. limit is 100")
	}
	transformers := make ([]repository.Transformer, 0, 105)
	for _, batchAction := range in.GetActions() {
		_, err := d.validateAction(ctx, batchAction)
		if err != nil {
			return nil, err
		}
		switch action := batchAction.Action.(type) {
		case *api.BatchAction_CreateEnvironmentLock:
			transformers = append(transformers, &repository.CreateEnvironmentLock{
				Environment: action.CreateEnvironmentLock.Environment,
				LockId:      action.CreateEnvironmentLock.LockId,
				Message:     action.CreateEnvironmentLock.Message,
			})
		case *api.BatchAction_DeleteEnvironmentLock:
			transformers = append(transformers, &repository.DeleteEnvironmentLock{
				Environment: action.DeleteEnvironmentLock.Environment,
				LockId:      action.DeleteEnvironmentLock.LockId,
			})
		case *api.BatchAction_CreateApplicationLock:
			transformers = append(transformers, &repository.CreateEnvironmentApplicationLock{
				Environment: action.CreateApplicationLock.Environment,
				Application: action.CreateApplicationLock.Application,
				LockId:      action.CreateApplicationLock.LockId,
				Message:     action.CreateApplicationLock.Message,
			})
		case *api.BatchAction_DeleteApplicationLock:
			transformers = append(transformers, &repository.DeleteEnvironmentApplicationLock{
				Environment: action.DeleteApplicationLock.Environment,
				Application: action.DeleteApplicationLock.Application,
				LockId:      action.DeleteApplicationLock.LockId,
			})
		case *api.BatchAction_Deploy:
			b := action.Deploy.LockBehavior
			if action.Deploy.IgnoreAllLocks {
				// the UI currently sets this to true,
				// in that case, we still want to ignore locks (for emergency deployments)
				b = api.LockBehavior_Ignore
			}
			transformers = append(transformers, &repository.DeployApplicationVersion{
				Environment:   action.Deploy.Environment,
				Application:   action.Deploy.Application,
				Version:       action.Deploy.Version,
				LockBehaviour: b,
			})
		}
	}

	err := d.Repository.Apply(ctx, transformers...)
	if err != nil {
		// TODO TE: error handling
		return nil, internalError(ctx, err)
	}
	return &emptypb.Empty{}, nil
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
