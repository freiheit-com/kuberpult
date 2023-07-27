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
	"fmt"
	"os"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/mapper"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/grpc"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type OverviewServiceServer struct {
	Repository repository.Repository
	Shutdown   <-chan struct{}

	notify notify.Notify

	init     sync.Once
	response atomic.Value
}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
	if in.GitRevision != "" {
		oid, err := git.NewOid(in.GitRevision)
		if err != nil {
			return nil, err
		}
		state, err := o.Repository.StateAt(oid)
		if err != nil {
			var gerr *git.GitError
			if errors.As(err, &gerr) {
				if gerr.Code == git.ErrNotFound {
					return nil, status.Error(codes.NotFound, "not found")
				}
			}
			return nil, err
		}
		return o.getOverview(ctx, state)
	}
	return o.getOverview(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {
	var rev string
	if s.Commit != nil {
		rev = s.Commit.Id().String()
	}
	result := api.GetOverviewResponse{
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
	}
	if envs, err := s.GetEnvironmentConfigs(); err != nil {
		return nil, grpc.InternalError(ctx, err)
	} else {
		result.EnvironmentGroups = mapper.MapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = mapper.DeriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(result.EnvironmentGroups, groupName, envName)
			env := api.Environment{
				Name: envName,
				Config: &api.EnvironmentConfig{
					Upstream:         mapper.TransformUpstream(config.Upstream),
					EnvironmentGroup: &groupName,
				},
				Locks:        map[string]*api.Lock{},
				Applications: map[string]*api.Environment_Application{},
			}
			if locks, err := s.GetEnvironmentLocks(envName); err != nil {
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
			if apps, err := s.GetEnvironmentApplications(envName); err != nil {
				return nil, err
			} else {
				for _, appName := range apps {
					app := api.Environment_Application{
						Name:               appName,
						Locks:              map[string]*api.Lock{},
						DeploymentMetaData: &api.Environment_Application_DeploymentMetaData{},
					}
					var version *uint64
					if version, err = s.GetEnvironmentApplicationVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {
						if version == nil {
							app.Version = 0
						} else {
							app.Version = *version
						}
					}
					if queuedVersion, err := s.GetQueuedVersion(envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
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
						if release, err := s.GetApplicationRelease(appName, *version); err != nil && !errors.Is(err, os.ErrNotExist) {
							return nil, err
						} else if release != nil {
							app.UndeployVersion = release.UndeployVersion
						}
					}
					if appLocks, err := s.GetEnvironmentApplicationLocks(envName, appName); err != nil {
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
							app.ArgoCD = &api.Environment_Application_ArgoCD{
								SyncWindows: syncWindows,
							}
						}
					}
					deployAuthor, deployTime, err := s.GetDeploymentMetaData(ctx, envName, appName)
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
	if apps, err := s.GetApplications(); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {
			app := api.Application{
				Name:          appName,
				Releases:      []*api.Release{},
				SourceRepoUrl: "",
				Team:          "",
			}
			if rels, err := s.GetApplicationReleases(appName); err != nil {
				return nil, err
			} else {
				for _, id := range rels {
					if rel, err := s.GetApplicationRelease(appName, id); err != nil {
						return nil, err
					} else {
						release := &api.Release{
							Version:         id,
							SourceAuthor:    rel.SourceAuthor,
							SourceCommitId:  rel.SourceCommitId,
							SourceMessage:   rel.SourceMessage,
							UndeployVersion: rel.UndeployVersion,
							CreatedAt:       timestamppb.New(rel.CreatedAt),
						}

						release.PrNumber = extractPrNumber(release.SourceMessage)

						app.Releases = append(app.Releases, release)
					}
				}
			}
			if team, err := s.GetApplicationTeamOwner(appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			if url, err := s.GetApplicationSourceRepoUrl(appName); err != nil {
				return nil, err
			} else {
				app.SourceRepoUrl = url
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
		return api.UndeploySummary_Undeploy
	}
	if allNormal {
		return api.UndeploySummary_Normal
	}
	return api.UndeploySummary_Mixed

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
		select {
		case <-ch:
			o.update(o.Repository.State())
		}
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
	r, err := o.getOverview(context.Background(), s)
	if err != nil {
		panic(err)
	}
	o.response.Store(r)
	o.notify.Notify()
}

func extractPrNumber(sourceMessage string) string {
	re := regexp.MustCompile("\\(#(\\d+)\\)")
	res := re.FindAllStringSubmatch(sourceMessage, -1)

	if len(res) == 0 {
		return ""
	} else {
		return res[len(res)-1][1]
	}
}
