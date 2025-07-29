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
	"errors"
	"fmt"
	"sync"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LockType string

const (
	LockTypeGo   LockType = "go"
	LockTypeDb   LockType = "db"
	LockTypeNone LockType = "none"
)

func ParseLockType(raw string) (LockType, error) {
	if raw == string(LockTypeDb) {
		return LockTypeDb, nil
	} else if raw == string(LockTypeGo) {
		return LockTypeGo, nil
	} else if raw == string(LockTypeNone) {
		return LockTypeNone, nil
	} else {
		return "", fmt.Errorf(
			"invalid lock type: '%s' - valid lock types are: '%s', '%s', '%s'",
			raw,
			string(LockTypeNone),
			string(LockTypeDb),
			string(LockTypeGo),
		)
	}
}

type BatchServerConfig struct {
	WriteCommitData      bool
	AllowedCILinkDomains []string //Transformers that create releases or deploy them can only accept CI links from these domains
	LockType             LockType
}

type BatchServer struct {
	Repository repository.Repository
	RBACConfig auth.RBACConfig
	Config     BatchServerConfig
	DBHandler  *db.DBHandler
}

// see maxBatchActions in store.tsx
const maxBatchActions int = 100

func ValidateEnvironmentLock(
	actionType string, // "create" | "delete"
	env types.EnvName,
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
	env types.EnvName,
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

func ValidateEnvironmentTeamLock(
	actionType string, // "create" | "delete"
	env string,
	team string,
	id string,
) error {

	return nil
}

func ValidateDeployment(
	env types.EnvName,
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

func ValidateEnvironment(
	environmentName types.EnvName,
	environmentConfig config.EnvironmentConfig,
) error {
	if environmentConfig.ArgoCdConfigs != nil && environmentConfig.ArgoCd != nil {
		return status.Error(codes.InvalidArgument, "specifying both argocd field and argo_configs is not supported")
	}
	if environmentConfig.ArgoCdConfigs == nil && environmentConfig.ArgoCd == nil {
		return status.Error(codes.InvalidArgument, "exactly one of the argocd or argo_configs fields must be specified")
	}
	if !valid.EnvironmentName(environmentName) {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("invalid environment name: '%s'", environmentName))
	}

	if environmentConfig.ArgoCdConfigs != nil {

		configs := environmentConfig.ArgoCdConfigs.ArgoCdConfigurations
		commonEnvPrefix := environmentConfig.ArgoCdConfigs.CommonEnvPrefix
		invalidNames := make([]types.EnvName, 0)
		knownNames := make(map[types.EnvName]struct{})

		if (commonEnvPrefix == nil || *commonEnvPrefix == "") && len(configs) > 1 {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("a common environment name prefix must be specified for active/active environments: '%s'", environmentName))
		}
		for _, currentConfig := range configs { //Each sub-environment must be valid
			var currentFullEnvironmentName types.EnvName
			if commonEnvPrefix == nil || *commonEnvPrefix == "" || len(configs) == 1 { //only 1 config provided, we dont care about prefixes or concrete env names
				currentFullEnvironmentName = types.EnvName(environmentName)
			} else {
				currentFullEnvironmentName = types.EnvName(*commonEnvPrefix + "-" + string(environmentName) + "-" + currentConfig.ConcreteEnvName)
			}

			if !valid.EnvironmentName(currentFullEnvironmentName) {
				invalidNames = append(invalidNames, currentFullEnvironmentName)
			}

			if _, exists := knownNames[currentFullEnvironmentName]; exists {
				return status.Error(codes.InvalidArgument, fmt.Sprintf("environment names must not be the same: %v", invalidNames))
			}
			knownNames[currentFullEnvironmentName] = struct{}{}
		}
		if len(invalidNames) != 0 {
			return status.Error(codes.InvalidArgument, fmt.Sprintf("one or more invalid environment names were provided: %v", invalidNames))
		}
	}
	return nil
}

func (d *BatchServer) processAction(
	batchAction *api.BatchAction,
) (repository.Transformer, *api.BatchResult, error) {
	switch action := batchAction.Action.(type) {
	case *api.BatchAction_CreateEnvironmentLock:
		act := action.CreateEnvironmentLock
		if err := ValidateEnvironmentLock("create", types.EnvName(act.Environment), act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.CreateEnvironmentLock{
			Environment:           types.EnvName(act.Environment),
			LockId:                act.LockId,
			Message:               act.Message,
			CiLink:                act.CiLink,
			SuggestedLifeTime:     act.SuggestedLifeTime,
			AllowedDomains:        d.Config.AllowedCILinkDomains,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_DeleteEnvironmentLock:
		act := action.DeleteEnvironmentLock
		if err := ValidateEnvironmentLock("delete", types.EnvName(act.Environment), act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.DeleteEnvironmentLock{
			Environment:           types.EnvName(act.Environment),
			LockId:                act.LockId,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_CreateEnvironmentApplicationLock:
		act := action.CreateEnvironmentApplicationLock
		if err := ValidateEnvironmentApplicationLock("create", types.EnvName(act.Environment), act.Application, act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.CreateEnvironmentApplicationLock{
			Environment:           types.EnvName(act.Environment),
			Application:           act.Application,
			LockId:                act.LockId,
			Message:               act.Message,
			CiLink:                act.CiLink,
			SuggestedLifeTime:     act.SuggestedLifeTime,
			AllowedDomains:        d.Config.AllowedCILinkDomains,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_DeleteEnvironmentApplicationLock:
		act := action.DeleteEnvironmentApplicationLock
		if err := ValidateEnvironmentApplicationLock("delete", types.EnvName(act.Environment), act.Application, act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.DeleteEnvironmentApplicationLock{
			Environment:           types.EnvName(act.Environment),
			Application:           act.Application,
			LockId:                act.LockId,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_CreateEnvironmentTeamLock:
		act := action.CreateEnvironmentTeamLock
		if err := ValidateEnvironmentTeamLock("create", act.Environment, act.Team, act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.CreateEnvironmentTeamLock{
			Environment:           types.EnvName(act.Environment),
			Team:                  act.Team,
			LockId:                act.LockId,
			Message:               act.Message,
			CiLink:                act.CiLink,
			SuggestedLifeTime:     act.SuggestedLifeTime,
			AllowedDomains:        d.Config.AllowedCILinkDomains,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_DeleteEnvironmentTeamLock:
		act := action.DeleteEnvironmentTeamLock
		if err := ValidateEnvironmentTeamLock("delete", act.Environment, act.Team, act.LockId); err != nil {
			return nil, nil, err
		}
		return &repository.DeleteEnvironmentTeamLock{
			Environment:           types.EnvName(act.Environment),
			Team:                  act.Team,
			LockId:                act.LockId,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_PrepareUndeploy:
		act := action.PrepareUndeploy
		if err := ValidateApplication(act.Application); err != nil {
			return nil, nil, err
		}
		return &repository.CreateUndeployApplicationVersion{
			Application:           act.Application,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			WriteCommitData:       d.Config.WriteCommitData,
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_Undeploy:
		act := action.Undeploy
		if err := ValidateApplication(act.Application); err != nil {
			return nil, nil, err
		}
		return &repository.UndeployApplication{
			Application:           act.Application,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_Deploy:
		act := action.Deploy
		if err := ValidateDeployment(types.EnvName(act.Environment), act.Application); err != nil {
			return nil, nil, err
		}
		b := act.LockBehavior
		if act.IgnoreAllLocks { //nolint: staticcheck
			// the UI currently sets this to true,
			// in that case, we still want to ignore locks (for emergency deployments)
			b = api.LockBehavior_IGNORE
		}
		return &repository.DeployApplicationVersion{
			SourceTrain:           nil,
			Environment:           types.EnvName(act.Environment),
			Application:           act.Application,
			Version:               act.Version,
			LockBehaviour:         b,
			WriteCommitData:       d.Config.WriteCommitData,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			Author:                "",
			CiLink:                "", //Only gets populated when a release is created or release train is conducted.
			TransformerEslVersion: 0,
			SkipCleanup:           false,
			Revision:              act.Revision,
		}, nil, nil
	case *api.BatchAction_DeleteEnvFromApp:
		act := action.DeleteEnvFromApp
		return &repository.DeleteEnvFromApp{
			Environment:           types.EnvName(act.Environment),
			Application:           act.Application,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_ReleaseTrain:
		in := action.ReleaseTrain
		if !valid.EnvironmentName(types.EnvName(in.Target)) {
			return nil, nil, status.Error(codes.InvalidArgument, "invalid environment")
		}
		if in.Team != "" && !valid.TeamName(in.Team) {
			return nil, nil, status.Error(codes.InvalidArgument, "invalid Team name")
		}
		return &repository.ReleaseTrain{
				Repo:                  d.Repository,
				Target:                in.Target,
				Team:                  in.Team,
				CommitHash:            in.CommitHash,
				WriteCommitData:       d.Config.WriteCommitData,
				TargetType:            in.TargetType.String(),
				Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
				TransformerEslVersion: 0,
				CiLink:                in.CiLink,
				AllowedDomains:        d.Config.AllowedCILinkDomains,
				GitTag:                types.GitTag(in.GitTag),
			}, &api.BatchResult{
				Result: &api.BatchResult_ReleaseTrain{
					ReleaseTrain: &api.ReleaseTrainResponse{Target: in.Target, Team: in.Team},
				},
			}, nil
	case *api.BatchAction_CreateRelease:
		in := action.CreateRelease
		response := api.CreateReleaseResponseSuccess{}
		downstreamEnvs := types.StringsToEnvNames(in.DeployToDownstreamEnvironments)
		if in.Team != "" && !valid.TeamName(in.Team) {
			return nil, nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid Team name: '%s'", in.Team))
		}
		return &repository.CreateApplicationVersion{
				Version:                        in.Version,
				Application:                    in.Application,
				Manifests:                      types.StringMapToEnvMap(in.Manifests),
				SourceCommitId:                 in.SourceCommitId,
				SourceAuthor:                   in.SourceAuthor,
				SourceMessage:                  in.SourceMessage,
				PreviousCommit:                 in.PreviousCommitId,
				Team:                           in.Team,
				DisplayVersion:                 in.DisplayVersion,
				Authentication:                 repository.Authentication{RBACConfig: d.RBACConfig},
				WriteCommitData:                d.Config.WriteCommitData,
				CiLink:                         in.CiLink,
				AllowedDomains:                 d.Config.AllowedCILinkDomains,
				TransformerEslVersion:          0,
				IsPrepublish:                   in.IsPrepublish,
				DeployToDownstreamEnvironments: downstreamEnvs,
				Revision:                       in.Revision,
			}, &api.BatchResult{
				Result: &api.BatchResult_CreateReleaseResponse{
					CreateReleaseResponse: &api.CreateReleaseResponse{
						Response: &api.CreateReleaseResponse_Success{
							Success: &response,
						},
					},
				},
			}, nil
	case *api.BatchAction_CreateEnvironment:
		in := action.CreateEnvironment
		conf := in.Config
		if conf == nil {
			//exhaustruct:ignore
			conf = &api.EnvironmentConfig{}
		}
		var argocd *config.EnvironmentConfigArgoCd
		if conf.Argocd != nil {
			argocd = transformArgoCdToConfig(conf.Argocd)
		}
		var configs *config.ArgoCDConfigs
		if conf.ArgoConfigs != nil {
			configs = transformArgoCdConfigsToConfig(conf.ArgoConfigs)
		}
		upstream := transformUpstreamToConfig(conf.Upstream)
		internalEnvironmentConfig := config.EnvironmentConfig{
			Upstream:         upstream,
			ArgoCd:           argocd,
			EnvironmentGroup: conf.EnvironmentGroup,
			ArgoCdConfigs:    configs,
		}
		if err := ValidateEnvironment(types.EnvName(in.Environment), internalEnvironmentConfig); err != nil {
			return nil, nil, status.Error(codes.InvalidArgument, fmt.Sprintf("processAction: invalid environment. err: %v", err))
		}
		transformer := &repository.CreateEnvironment{
			Environment:           types.EnvName(in.Environment),
			Config:                internalEnvironmentConfig,
			TransformerEslVersion: 0,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
		}
		return transformer, nil, nil
	case *api.BatchAction_CreateEnvironmentGroupLock:
		act := action.CreateEnvironmentGroupLock
		return &repository.CreateEnvironmentGroupLock{
			EnvironmentGroup:      act.EnvironmentGroup,
			LockId:                act.LockId,
			Message:               act.Message,
			CiLink:                act.CiLink,
			SuggestedLifeTime:     act.SuggestedLifeTime,
			AllowedDomains:        d.Config.AllowedCILinkDomains,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_DeleteEnvironmentGroupLock:
		act := action.DeleteEnvironmentGroupLock
		return &repository.DeleteEnvironmentGroupLock{
			EnvironmentGroup:      act.EnvironmentGroup,
			LockId:                act.LockId,
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	case *api.BatchAction_DeleteEnvironment:
		act := action.DeleteEnvironment
		return &repository.DeleteEnvironment{
			Environment:           types.EnvName(act.Environment),
			Authentication:        repository.Authentication{RBACConfig: d.RBACConfig},
			TransformerEslVersion: 0,
		}, nil, nil
	}

	return nil, nil, status.Error(codes.InvalidArgument, "processAction: cannot process action: invalid action type")
}

var isolatedTransformersLock sync.RWMutex
var isolatedTransformerNames = []db.EventType{db.EvtUndeployApplication, db.EvtDeleteEnvFromApp, db.EvtDeleteEnvironment}

func (d *BatchServer) ProcessBatch(
	ctx context.Context,
	in *api.BatchRequest,
) (*api.BatchResponse, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "ProcessBatch")
	defer span.Finish()
	span.SetTag("BatchActions", len(in.GetActions()))
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return nil, onErr(grpc.AuthError(ctx, fmt.Errorf("batch requires user to be provided %v", err)))
	}
	ctx = auth.WriteUserToContext(ctx, *user)
	if len(in.GetActions()) > maxBatchActions {
		return nil, onErr(status.Error(codes.InvalidArgument, fmt.Sprintf("cannot process batch: too many actions. limit is %d", maxBatchActions)))
	}

	results := make([]*api.BatchResult, 0, len(in.GetActions()))
	transformers := make([]repository.Transformer, 0, maxBatchActions)
	requiresIsolation := false
	for _, batchAction := range in.GetActions() {
		transformer, result, err := d.processAction(batchAction)
		if err != nil {
			// Validation error
			return nil, onErr(err)
		}

		transformerTypeName := transformer.GetDBEventType()
		for _, isolatedTransformerName := range isolatedTransformerNames {
			if isolatedTransformerName == transformerTypeName {
				requiresIsolation = true
			}
		}
		transformers = append(transformers, transformer)
		results = append(results, result)
	}
	if requiresIsolation {
		// we protect all "destructive" operations by a read-write lock, so that only 1 destructive operation can be run in parallel:
		isolationSpan, _, _ := tracing.StartSpanFromContext(ctx, "Wait-Lock")
		if d.Config.LockType == LockTypeGo {
			// This solution (go locks) is not scalable and doesn't work when we have multiple cd-service pods
			isolatedTransformersLock.Lock()
			defer isolatedTransformersLock.Unlock()
		}
		isolationSpan.Finish()
	} else {
		// we also use a read lock, so that destructive and non-destructive transformers cannot run in parallel:
		isolationSpan, _, _ := tracing.StartSpanFromContext(ctx, "Wait-RLock")
		if d.Config.LockType == LockTypeGo {
			isolatedTransformersLock.RLock()
			defer isolatedTransformersLock.RUnlock()
		}
		isolationSpan.Finish()
	}

	if d.Config.LockType == LockTypeDb {
		isShared := !requiresIsolation
		err = d.DBHandler.WithAdvisoryLock(ctx, isShared, db.LockIsolateTransformers, func(ctx context.Context) error {
			return d.Repository.Apply(ctx, transformers...)
		})
	} else {
		if d.Config.LockType == LockTypeNone {
			logger.FromContext(ctx).Sugar().Warnf("not locking at all")
		}
		err = d.Repository.Apply(ctx, transformers...)
	}
	if err != nil {
		logger.FromContext(ctx).Sugar().Warnf("error in Repository.Apply: %v", err)
		if errors.Is(err, repository.ErrQueueFull) {
			return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf("Could not process ProcessBatch request. Err: %s", err.Error()))
		}
		var applyErr = repository.UnwrapUntilTransformerBatchApplyError(err)
		if applyErr != nil {
			resp, handledErr := d.handleError(applyErr, err)
			return resp, onErr(handledErr)
		}
		return nil, onErr(err)
	}
	return &api.BatchResponse{Results: results}, nil
}

func (d *BatchServer) handleError(applyErr *repository.TransformerBatchApplyError, err error) (*api.BatchResponse, error) {
	switch transformerError := applyErr.TransformerError.(type) {
	case *repository.CreateReleaseError:
		{
			errorResults := make([]*api.BatchResult, 1)
			errorResults[0] = &api.BatchResult{
				Result: &api.BatchResult_CreateReleaseResponse{
					CreateReleaseResponse: transformerError.Response(),
				},
			}
			return &api.BatchResponse{Results: errorResults}, nil
		}
	case *repository.TeamNotFoundErr:
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("Could not process ProcessBatch request. Err: %s", applyErr.TransformerError.Error()))
	default:
		tmp, ok := status.FromError(applyErr.TransformerError)
		if tmp != nil && ok {
			// in order to pass the right status code, we need to return the inner error:
			return nil, applyErr.TransformerError
		}
		return nil, err
	}
}

var _ api.BatchServiceServer = (*BatchServer)(nil)
