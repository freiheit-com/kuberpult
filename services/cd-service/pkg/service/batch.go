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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type BatchServer struct {
	Repository repository.Repository
}

const maxBatchActions int = 100

func ValidateEnvironmentLock(
	actionType string, // "create" | "delete"
	env string,
	id string,
) error {
	if !valid.EnvironmentName(env) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot %s environment lock: invalid environment: '%s'", actionType, env))
	}
	if !valid.LockId(id) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot %s environment lock: invalid lock id: '%s'", actionType, id))
	}
	return nil
}

func ValidateEnvironmentApplicationLock(
	actionType string, // "create" | "delete"
	env string,
	app string,
	id string,
) error {
	if !valid.EnvironmentName(env) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot %s environment application lock: invalid environment: '%s'", actionType, env))
	}
	if !valid.ApplicationName(app) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot %s environment application lock: invalid application: '%s'", actionType, app))
	}
	if !valid.LockId(id) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot %s environment application lock: invalid lock id: '%s'", actionType, id))
	}
	return nil
}

func ValidateDeployment(
	env string,
	app string,
) error {
	if !valid.EnvironmentName(env) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot deploy environment application lock: invalid environment: '%s'", env))
	}
	if !valid.ApplicationName(app) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot deploy environment application lock: invalid application: '%s'", app))
	}
	return nil
}

func ValidateApplication(
	app string,
) error {
	if !valid.ApplicationName(app) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("cannot create undeploy version: invalid application: '%s'", app))
	}
	return nil
}

func (d *BatchServer) processAction(
	batchAction *api.BatchAction,
) (repository.Transformer, error) {
	switch action := batchAction.Action.(type) {
	case *api.BatchAction_CreateEnvironmentLock:
		act := action.CreateEnvironmentLock
		if err := ValidateEnvironmentLock("create", act.Environment, act.LockId); err != nil {
			return nil, err
		}
		return &repository.CreateEnvironmentLock{
			Environment: act.Environment,
			LockId:      act.LockId,
			Message:     act.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentLock:
		act := action.DeleteEnvironmentLock
		if err := ValidateEnvironmentLock("delete", act.Environment, act.LockId); err != nil {
			return nil, err
		}
		return &repository.DeleteEnvironmentLock{
			Environment: act.Environment,
			LockId:      act.LockId,
		}, nil
	case *api.BatchAction_CreateEnvironmentApplicationLock:
		act := action.CreateEnvironmentApplicationLock
		if err := ValidateEnvironmentApplicationLock("create", act.Environment, act.Application, act.LockId); err != nil {
			return nil, err
		}
		return &repository.CreateEnvironmentApplicationLock{
			Environment: act.Environment,
			Application: act.Application,
			LockId:      act.LockId,
			Message:     act.Message,
		}, nil
	case *api.BatchAction_DeleteEnvironmentApplicationLock:
		act := action.DeleteEnvironmentApplicationLock
		if err := ValidateEnvironmentApplicationLock("delete", act.Environment, act.Application, act.LockId); err != nil {
			return nil, err
		}
		return &repository.DeleteEnvironmentApplicationLock{
			Environment: act.Environment,
			Application: act.Application,
			LockId:      act.LockId,
		}, nil
	case *api.BatchAction_PrepareUndeploy:
		act := action.PrepareUndeploy
		if err := ValidateApplication(act.Application); err != nil {
			return nil, err
		}
		return &repository.CreateUndeployApplicationVersion{
			Application: act.Application,
		}, nil
	case *api.BatchAction_Undeploy:
		act := action.Undeploy
		if err := ValidateApplication(act.Application); err != nil {
			return nil, err
		}
		return &repository.UndeployApplication{
			Application: act.Application,
		}, nil
	case *api.BatchAction_Deploy:
		act := action.Deploy
		if err := ValidateDeployment(act.Environment, act.Application); err != nil {
			return nil, err
		}
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
	}
	return nil, status.Error(codes.InvalidArgument, "cannot process action: invalid action type")
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
		transformer, err := d.processAction(batchAction)
		if err != nil {
			// Validation error
			return nil, err
		}
		transformers = append(transformers, transformer)
	}
	err := d.Repository.Apply(ctx, transformers...)
	if err != nil {
		return nil, httperrors.InternalError(ctx, fmt.Errorf("could not apply transformer: %w", err))
	}
	return &emptypb.Empty{}, nil
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
