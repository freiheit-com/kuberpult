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
	"os"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/pkg/mapper"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"

	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type OverviewServiceServer struct {
	Repository       repository.Repository
	RepositoryConfig repository.RepositoryConfig
	Shutdown         <-chan struct{}

	notify notify.Notify

	init     sync.Once
	response atomic.Value
}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	if !o.Repository.State().DBHandler.ShouldUseEslTable() && in.GitRevision != "" {
		oid, err := git.NewOid(in.GitRevision)
		if err != nil {
			return nil, grpc.PublicError(ctx, fmt.Errorf("getOverview: could not find revision %v: %v", in.GitRevision, err))
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
		return o.getOverviewDB(ctx, state)
	}
	return o.getOverviewDB(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getOverviewDB(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {

	if s.DBHandler.ShouldUseOtherTables() {
		response, err := db.WithTransactionT[api.GetOverviewResponse](s.DBHandler, ctx, false, func(ctx context.Context, transaction *sql.Tx) (*api.GetOverviewResponse, error) {
			var err2 error
			cached_result, err2 := s.DBHandler.ReadLatestOverviewCache(ctx, transaction)
			if err2 != nil {
				return nil, err2
			}
			if !s.DBHandler.IsOverviewEmpty(cached_result) {
				return cached_result, nil
			}

			response, err2 := o.getOverview(ctx, s, transaction)
			if err2 != nil {
				return nil, err2
			}
			err2 = s.DBHandler.WriteOverviewCache(ctx, transaction, response)
			if err2 != nil {
				return nil, err2
			}
			return response, nil
		})
		if err != nil {
			return nil, err
		}
		return response, nil
	}
	return o.getOverview(ctx, s, nil)
}

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State,
	transaction *sql.Tx,
) (*api.GetOverviewResponse, error) {
	var rev string
	if s.DBHandler.ShouldUseOtherTables() {
		rev = "0000000000000000000000000000000000000000"
	} else {
		if s.Commit != nil {
			rev = s.Commit.Id().String()
		}
	}
	result := api.GetOverviewResponse{
		Branch:            "",
		ManifestRepoUrl:   "",
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
	}
	result.ManifestRepoUrl = o.RepositoryConfig.URL
	result.Branch = o.RepositoryConfig.Branch
	if envs, err := s.GetAllEnvironmentConfigs(ctx, transaction); err != nil {
		return nil, grpc.InternalError(ctx, err)
	} else {
		result.EnvironmentGroups = mapper.MapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = mapper.DeriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(result.EnvironmentGroups, groupName, envName)
			//exhaustruct:ignore
			argocd := &api.EnvironmentConfig_ArgoCD{}
			if config.ArgoCd != nil {
				argocd = mapper.TransformArgocd(*config.ArgoCd)
			}
			env := api.Environment{
				DistanceToUpstream: 0,
				Priority:           api.Priority_PROD,
				Name:               envName,
				Config: &api.EnvironmentConfig{
					Upstream:         mapper.TransformUpstream(config.Upstream),
					Argocd:           argocd,
					EnvironmentGroup: &groupName,
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			envInGroup.Config = env.Config
			if locks, err := s.GetEnvironmentLocks(ctx, transaction, envName); err != nil {
				return nil, err
			} else {
				for lockId, lock := range locks {
					env.Locks[lockId] = &api.Lock{
						Message:   lock.Message,
						LockId:    lockId,
						CreatedAt: timestamppb.New(lock.CreatedAt),
						CreatedBy: &api.Actor{
							Name:  lock.CreatedBy.Name,
							Email: lock.CreatedBy.Email,
						},
					}
				}
				envInGroup.Locks = env.Locks
			}

			if apps, err := s.GetEnvironmentApplications(ctx, transaction, envName); err != nil {
				return nil, err
			} else {
				for _, appName := range apps {
					teamName, err := s.GetTeamName(ctx, transaction, appName)
					app := api.Environment_Application{
						Version:         0,
						QueuedVersion:   0,
						UndeployVersion: false,
						ArgoCd:          nil,
						Name:            appName,
						Locks:           map[string]*api.Lock{},
						TeamLocks:       map[string]*api.Lock{},
						Team:            teamName,
						DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{
							DeployAuthor: "",
							DeployTime:   "",
						},
					}
					if err == nil {
						if teamLocks, teamErr := s.GetEnvironmentTeamLocks(ctx, transaction, envName, teamName); teamErr != nil {
							return nil, teamErr
						} else {
							for lockId, lock := range teamLocks {
								app.TeamLocks[lockId] = &api.Lock{
									Message:   lock.Message,
									LockId:    lockId,
									CreatedAt: timestamppb.New(lock.CreatedAt),
									CreatedBy: &api.Actor{
										Name:  lock.CreatedBy.Name,
										Email: lock.CreatedBy.Email,
									},
								}
							}
						}
					} // Err != nil means no team name was found so no need to parse team locks

					var version *uint64
					version, err = s.GetEnvironmentApplicationVersion(ctx, transaction, envName, appName)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {

						if version == nil {
							app.Version = 0
						} else {
							app.Version = *version
						}
					}

					if queuedVersion, err := s.GetQueuedVersion(ctx, transaction, envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if queuedVersion == nil {
							app.QueuedVersion = 0
						} else {
							app.QueuedVersion = *queuedVersion
						}
					}
					app.UndeployVersion = false
					if version != nil {
						if release, err := s.GetApplicationRelease(ctx, transaction, appName, *version); err != nil && !errors.Is(err, os.ErrNotExist) {
							return nil, err
						} else if release != nil {
							app.UndeployVersion = release.UndeployVersion
						}
					}

					if appLocks, err := s.GetEnvironmentApplicationLocks(ctx, transaction, envName, appName); err != nil {
						return nil, err
					} else {
						for lockId, lock := range appLocks {
							app.Locks[lockId] = &api.Lock{
								Message:   lock.Message,
								LockId:    lockId,
								CreatedAt: timestamppb.New(lock.CreatedAt),
								CreatedBy: &api.Actor{
									Name:  lock.CreatedBy.Name,
									Email: lock.CreatedBy.Email,
								},
							}
						}
					}
					if config.ArgoCd != nil {
						if syncWindows, err := mapper.TransformSyncWindows(config.ArgoCd.SyncWindows, appName); err != nil {
							return nil, err
						} else {
							app.ArgoCd = &api.Environment_Application_ArgoCD{
								SyncWindows: syncWindows,
							}
						}
					}
					deployAuthor, deployTime, err := s.GetDeploymentMetaData(ctx, transaction, envName, appName)
					if err != nil {
						return nil, err
					}
					app.DeploymentMetaData.DeployAuthor = deployAuthor
					if deployTime.IsZero() {
						app.DeploymentMetaData.DeployTime = ""
					} else {
						app.DeploymentMetaData.DeployTime = fmt.Sprintf("%d", deployTime.Unix())
					}
					env.Applications[appName] = &app
				}
			}
			envInGroup.Applications = env.Applications
		}
	}
	if apps, err := s.GetApplications(ctx, transaction); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {
			app := api.Application{
				UndeploySummary: 0,
				Warnings:        nil,
				Name:            appName,
				Releases:        []*api.Release{},
				SourceRepoUrl:   "",
				Team:            "",
			}
			if rels, err := s.GetAllApplicationReleases(ctx, transaction, appName); err != nil {
				return nil, err
			} else {
				for _, id := range rels {
					if rel, err := s.GetApplicationRelease(ctx, transaction, appName, id); err != nil {
						return nil, err
					} else {
						if rel == nil {
							// ignore
						} else {
							release := rel.ToProto()
							release.Version = id
							release.UndeployVersion = rel.UndeployVersion
							app.Releases = append(app.Releases, release)
						}
					}
				}
			}
			if team, err := s.GetApplicationTeamOwner(ctx, transaction, appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			app.UndeploySummary = deriveUndeploySummary(appName, result.EnvironmentGroups)
			app.Warnings = CalculateWarnings(ctx, app.Name, result.EnvironmentGroups)
			result.Applications[appName] = &app
		}

	}

	return &result, nil
}

/*
CalculateWarnings returns warnings for the User to be displayed in the UI.
For really unusual configurations, these will be logged and not returned.
*/
func CalculateWarnings(ctx context.Context, appName string, groups []*api.EnvironmentGroup) []*api.Warning {
	result := make([]*api.Warning, 0)
	for e := 0; e < len(groups); e++ {
		group := groups[e]
		for i := 0; i < len(groups[e].Environments); i++ {
			env := group.Environments[i]
			if env.Config.Upstream == nil || env.Config.Upstream.Environment == nil {
				// if the env has no upstream, there's nothing to warn about
				continue
			}
			upstreamEnvName := env.Config.GetUpstream().Environment
			upstreamEnv := getEnvironmentByName(groups, *upstreamEnvName)
			if upstreamEnv == nil {
				// this is already checked on startup and therefore shouldn't happen here
				continue
			}

			appInEnv := env.Applications[appName]
			if appInEnv == nil {
				// appName is not deployed here, ignore it
				continue
			}
			versionInEnv := appInEnv.Version
			appInUpstreamEnv := upstreamEnv.Applications[appName]
			if appInUpstreamEnv == nil {
				// appName is not deployed upstream... that's unusual!
				var warning = api.Warning{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         versionInEnv,
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
				continue
			}
			versionInUpstreamEnv := appInUpstreamEnv.Version

			if versionInEnv > versionInUpstreamEnv && len(appInEnv.Locks) == 0 {
				var warning = api.Warning{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     versionInUpstreamEnv,
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         versionInEnv,
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
			}
		}
	}
	return result
}

func deriveUndeploySummary(appName string, groups []*api.EnvironmentGroup) api.UndeploySummary {
	var allNormal = true
	var allUndeploy = true
	for _, group := range groups {
		for _, environment := range group.Environments {
			var app, exists = environment.Applications[appName]
			if !exists {
				continue
			}
			if app.Version == 0 {
				// if the app exists but nothing is deployed, we ignore this
				continue
			}
			if app.UndeployVersion {
				allNormal = false
			} else {
				allUndeploy = false
			}
		}
	}
	if allUndeploy {
		return api.UndeploySummary_UNDEPLOY
	}
	if allNormal {
		return api.UndeploySummary_NORMAL
	}
	return api.UndeploySummary_MIXED

}

func getEnvironmentInGroup(groups []*api.EnvironmentGroup, groupNameToReturn string, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		if currentGroup.EnvironmentGroupName == groupNameToReturn {
			for _, currentEnv := range currentGroup.Environments {
				if currentEnv.Name == envNameToReturn {
					return currentEnv
				}
			}
		}
	}
	return nil
}

func getEnvironmentByName(groups []*api.EnvironmentGroup, envNameToReturn string) *api.Environment {
	for _, currentGroup := range groups {
		for _, currentEnv := range currentGroup.Environments {
			if currentEnv.Name == envNameToReturn {
				return currentEnv
			}
		}
	}
	return nil
}

func (o *OverviewServiceServer) StreamOverview(in *api.GetOverviewRequest,
	stream api.OverviewService_StreamOverviewServer) error {
	ch, unsubscribe := o.subscribe()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-o.Shutdown:
			return nil
		case <-ch:
			ov := o.response.Load().(*api.GetOverviewResponse)
			if err := stream.Send(ov); err != nil {
				// if we don't log this here, the details will be lost - so this is an exception to the rule "either return an error or log it".
				// for example if there's an invalid encoding, grpc will just give a generic error like
				// "error while marshaling: string field contains invalid UTF-8"
				// but it won't tell us which field has the issue. This is then very hard to debug further.
				logger.FromContext(stream.Context()).Error("error sending overview response:", zap.Error(err), zap.String("overview", fmt.Sprintf("%+v", ov)))
				return err
			}

		case <-done:
			return nil
		}
	}
}

func (o *OverviewServiceServer) subscribe() (<-chan struct{}, notify.Unsubscribe) {
	o.init.Do(func() {
		ch, unsub := o.Repository.Notify().Subscribe()
		// Channels obtained from subscribe are by default triggered
		//
		// This means, we have to wait here until the first overview is loaded.
		<-ch
		o.update(o.Repository.State())
		go func() {
			defer unsub()
			for {
				select {
				case <-o.Shutdown:
					return
				case <-ch:
					o.update(o.Repository.State())
				}
			}
		}()
	})
	return o.notify.Subscribe()
}

func (o *OverviewServiceServer) update(s *repository.State) {
	r, err := o.getOverviewDB(context.Background(), s)
	if err != nil {
		panic(err)
	}
	o.response.Store(r)
	o.notify.Notify()
}
