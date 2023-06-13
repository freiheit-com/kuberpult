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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/httperrors"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BatchServer struct {
	Repository repository.Repository
}

const maxBatchActions int = 100

func (d *BatchServer) processAction(
	ctx context.Context,
	batchAction *api.BatchAction,
) (repository.Transformer, error) {
	switch action := batchAction.Action.(type) {
	case *api.BatchAction_CreateEnvironmentLock:
		act := action.CreateEnvironmentLock
		return &repository.CreateEnvironmentLock{
			Environment: act.Environment,
			LockId:      act.LockId,
			Message:     act.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentLock:
		act := action.DeleteEnvironmentLock
		return &repository.DeleteEnvironmentLock{
			Environment: act.Environment,
			LockId:      act.LockId,
		}, nil
	case *api.BatchAction_CreateEnvironmentApplicationLock:
		act := action.CreateEnvironmentApplicationLock
		return &repository.CreateEnvironmentApplicationLock{
			Environment: act.Environment,
			Application: act.Application,
			LockId:      act.LockId,
			Message:     act.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentApplicationLock:
		act := action.DeleteEnvironmentApplicationLock
		return &repository.DeleteEnvironmentApplicationLock{
			Environment: act.Environment,
			Application: act.Application,
			LockId:      act.LockId,
		}, nil
	case *api.BatchAction_PrepareUndeploy:
		act := action.PrepareUndeploy
		return &repository.CreateUndeployApplicationVersion{
			Application: act.Application,
		}, nil
	case *api.BatchAction_Undeploy:
		act := action.Undeploy
		return &repository.UndeployApplication{
			Application: act.Application,
		}, nil
	case *api.BatchAction_Deploy:
		act := action.Deploy
		b := act.LockBehavior
		if act.IgnoreAllLocks {
			// the UI currently sets this to true,
			// in that case, we still want to ignore locks (for emergency deployments)
			b = api.LockBehavior_Ignore
		}
		return &repository.DeployApplicationVersion{
			Environment:   act.Environment,
			Application:   act.Application,
			Version:       act.Version,
			LockBehaviour: b,
		}, nil
	case *api.BatchAction_DeleteEnvFromApp:
		act := action.DeleteEnvFromApp
		return &repository.DeleteEnvFromApp{
			Environment: act.Environment,
			Application: act.Application,
		}, nil
	default:
		//action
		return nil, httperrors.InvalidArgumentError(ctx, fmt.Errorf("processAction: cannot process action: invalid action type %T", action))
	}
}

func (d *BatchServer) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest,
) (*emptypb.Empty, error) {
	if len(in.GetActions()) > maxBatchActions {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("cannot process batch: too many actions. limit is %d", maxBatchActions))
	}
	transformers := make([]repository.Transformer, 0, maxBatchActions)
	for _, batchAction := range in.GetActions() {
		transformer, err := d.processAction(ctx, batchAction)
		if err != nil {
			return nil, err
		}
		transformers = append(transformers, transformer)
	}
	err := d.Repository.Apply(ctx, transformers...)
	if err != nil {
		statusErr, ok := status.FromError(err)
		if ok && statusErr != nil {
			// if it is a status error, we return to the caller without change:
			return nil, err
		}
		// otherwise, we use InternalError to print the error and return just a generic error message to the caller:
		return nil, httperrors.InternalError(ctx, fmt.Errorf("could not apply transformer: %w", err))
	}
	return &emptypb.Empty{}, nil
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
