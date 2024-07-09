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
/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com
*/
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func UpdateOverviewDB(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
) error {
	var rev string
	if state.Commit != nil {
		rev = state.Commit.Id().String()
	}
	result := &api.GetOverviewResponse{
		Branch:            "",
		ManifestRepoUrl:   "",
		Applications:      map[string]*api.Application{},
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
	}
	latestOverviewRow, err := state.DBHandler.ReadLatestOverviewCache(ctx, transaction)
	if err != nil {
		return err
	}

	if latestOverviewRow != nil {
		err = json.Unmarshal([]byte(latestOverviewRow.Blob), result)
		if err != nil {
			return err
		}
	}
	result, err = UpdateOverviewEnvironmentGroups(ctx, state, transaction, result)
	if err != nil {
		return err
	}
	result, err = UpdateOverviewApplications(ctx, state, transaction, result)
	if err != nil {
		return err
	}
	resultBlob, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return state.DBHandler.WriteOverviewCache(ctx, transaction, string(resultBlob))

}

func UpdateOverviewApplications(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
	previousOverview *api.GetOverviewResponse,
) (*api.GetOverviewResponse, error) {
	previousOverview.Applications = map[string]*api.Application{}
	if apps, err := state.GetApplications(ctx, transaction); err != nil {
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
			if rels, err := state.GetAllApplicationReleases(ctx, transaction, appName); err != nil {
				return nil, err
			} else {
				for _, id := range rels {
					if rel, err := state.GetApplicationRelease(ctx, transaction, appName, id); err != nil {
						return nil, err
					} else {
						if rel == nil {
							// ignore
						} else {
							release := rel.ToProto()
							release.Version = id

							app.Releases = append(app.Releases, release)
						}
					}
				}
			}
			if team, err := state.GetApplicationTeamOwner(ctx, transaction, appName); err != nil {
				return nil, err
			} else {
				app.Team = team
			}
			if url, err := state.GetApplicationSourceRepoUrl(appName); err != nil {
				return nil, err
			} else {
				app.SourceRepoUrl = url
			}
			app.UndeploySummary = DeriveUndeploySummary(appName, previousOverview.EnvironmentGroups)
			app.Warnings = CalculateWarnings(ctx, app.Name, previousOverview.EnvironmentGroups)
			previousOverview.Applications[appName] = &app
		}

	}
	return previousOverview, nil
}

func UpdateOverviewEnvironmentGroups(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
	previousOverview *api.GetOverviewResponse,
) (*api.GetOverviewResponse, error) {

	if envs, err := state.GetAllEnvironmentConfigs(ctx, transaction); err != nil {
		return nil, grpc.InternalError(ctx, err)
	} else {
		previousOverview.EnvironmentGroups = mapper.MapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = mapper.DeriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(previousOverview.EnvironmentGroups, groupName, envName)
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
			if locks, err := state.GetEnvironmentLocks(ctx, transaction, envName); err != nil {
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

			if apps, err := state.GetEnvironmentApplications(ctx, transaction, envName); err != nil {
				return nil, err
			} else {

				for _, appName := range apps {
					teamName, err := state.GetTeamName(ctx, transaction, appName)
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
						if teamLocks, teamErr := state.GetEnvironmentTeamLocks(ctx, transaction, envName, teamName); teamErr != nil {
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
					version, err = state.GetEnvironmentApplicationVersion(ctx, transaction, envName, appName)
					if err != nil && !errors.Is(err, os.ErrNotExist) {
						return nil, err
					} else {

						if version == nil {
							app.Version = 0
						} else {
							app.Version = *version
						}
					}

					if queuedVersion, err := state.GetQueuedVersion(ctx, transaction, envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
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
						if release, err := state.GetApplicationRelease(ctx, transaction, appName, *version); err != nil && !errors.Is(err, os.ErrNotExist) {
							return nil, err
						} else if release != nil {
							app.UndeployVersion = release.UndeployVersion
						}
					}

					if appLocks, err := state.GetEnvironmentApplicationLocks(ctx, transaction, envName, appName); err != nil {
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
					deployAuthor, deployTime, err := state.GetDeploymentMetaData(ctx, transaction, envName, appName)
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

	return previousOverview, nil
}

func DeriveUndeploySummary(appName string, groups []*api.EnvironmentGroup) api.UndeploySummary {
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
