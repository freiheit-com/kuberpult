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
	"github.com/freiheit-com/kuberpult/pkg/types"

	"fmt"
	"slices"

	"sync"
	"sync/atomic"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type OverviewServiceServer struct {
	Repository       repository.Repository
	RepositoryConfig repository.RepositoryConfig
	Shutdown         <-chan struct{}

	notify                       notify.Notify
	Context                      context.Context
	overviewStreamingInitFunc    sync.Once
	changedAppsStreamingInitFunc sync.Once
	response                     atomic.Value

	DBHandler *db.DBHandler
}

func (o *OverviewServiceServer) GetAppDetails(
	ctx context.Context,
	in *api.GetAppDetailsRequest) (*api.GetAppDetailsResponse, error) {

	span, ctx := tracer.StartSpanFromContext(ctx, "GetAppDetails")
	defer span.Finish()
	span.SetTag("application", in.AppName)

	var appName = in.AppName
	var response = &api.GetAppDetailsResponse{
		Application: &api.Application{
			UndeploySummary: 0,
			Warnings:        nil,
			Name:            appName,
			Releases:        []*api.Release{},
			SourceRepoUrl:   "",
			Team:            "",
		},
		AppLocks:    make(map[string]*api.Locks),
		Deployments: make(map[string]*api.Deployment),
		TeamLocks:   make(map[string]*api.Locks),
	}
	resultApp, err := db.WithTransactionT(o.DBHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*api.Application, error) {
		var rels []types.ReleaseNumbers
		var result = &api.Application{
			UndeploySummary: 0,
			Warnings:        nil,
			Name:            appName,
			Releases:        []*api.Release{},
			SourceRepoUrl:   "",
			Team:            "",
		}

		// Releases
		result.Name = appName
		retrievedReleasesOfApp, err := o.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("app without releases: %v", err)
		}
		if retrievedReleasesOfApp != nil {
			rels = retrievedReleasesOfApp
		}

		uintRels := make([]uint64, len(rels))
		for idx, r := range rels {
			uintRels[idx] = *r.Version
		}
		//Does not get the manifest and gets all releases at the same time
		releases, err := o.DBHandler.DBSelectReleasesByVersionsAndRevision(ctx, transaction, appName, uintRels, false)
		if err != nil {
			return nil, err
		}

		for _, currentRelease := range releases {
			var tmp = &repository.Release{
				Version:         *currentRelease.ReleaseNumbers.Version,
				UndeployVersion: currentRelease.Metadata.UndeployVersion,
				SourceAuthor:    currentRelease.Metadata.SourceAuthor,
				SourceCommitId:  currentRelease.Metadata.SourceCommitId,
				SourceMessage:   currentRelease.Metadata.SourceMessage,
				CreatedAt:       currentRelease.Created,
				DisplayVersion:  currentRelease.Metadata.DisplayVersion,
				IsMinor:         currentRelease.Metadata.IsMinor,
				IsPrepublish:    currentRelease.Metadata.IsPrepublish,
				Environments:    currentRelease.Environments,
				CiLink:          currentRelease.Metadata.CiLink,
				Revision:        currentRelease.ReleaseNumbers.Revision,
			}
			result.Releases = append(result.Releases, tmp.ToProto())
		}

		appTeamName, err := o.Repository.State().GetApplicationTeamOwner(ctx, transaction, appName)
		if err != nil {
			return nil, fmt.Errorf("app team not found: %s", appName)
		}
		result.Team = appTeamName

		if response == nil {
			return nil, fmt.Errorf("app not found: '%s'", appName)
		}
		dbAllEnvs, err := o.DBHandler.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve all environments, error: %w", err)
		}
		if dbAllEnvs == nil {
			return nil, nil
		}
		envs, err := o.DBHandler.DBSelectEnvironmentsBatch(ctx, transaction, dbAllEnvs)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve manifests for environments %v from the database, error: %w", dbAllEnvs, err)
		}

		envMap := make(map[types.EnvName]db.DBEnvironment)
		envConfigs := make(map[types.EnvName]config.EnvironmentConfig)
		for _, env := range *envs {
			envMap[env.Name] = env
			envConfigs[env.Name] = env.Config
		}

		envGroups := mapper.MapEnvironmentsToGroups(envConfigs)

		// App Locks
		appLocks, err := o.DBHandler.DBSelectAllActiveAppLocksForApp(ctx, transaction, appName)
		if err != nil {
			return nil, fmt.Errorf("could not find application locks for app %s: %w", appName, err)
		}
		for _, currentLock := range appLocks {
			e := string(currentLock.Env)
			if _, ok := response.AppLocks[e]; !ok {
				response.AppLocks[e] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}
			response.AppLocks[e].Locks = append(response.AppLocks[e].Locks, &api.Lock{
				LockId:    currentLock.LockID,
				Message:   currentLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentLock.Metadata.CreatedByName,
					Email: currentLock.Metadata.CreatedByEmail,
				},
				CiLink:            currentLock.Metadata.CiLink,
				SuggestedLifetime: currentLock.Metadata.SuggestedLifeTime,
			})
		}

		// Team Locks
		teamLocks, err := o.DBHandler.DBSelectAllActiveTeamLocksForTeam(ctx, transaction, result.Team)
		if err != nil {
			return nil, fmt.Errorf("could not find team locks for app %s: %w", appName, err)
		}
		for _, currentTeamLock := range teamLocks {
			e := string(currentTeamLock.Env)
			if _, ok := response.TeamLocks[e]; !ok {
				response.TeamLocks[e] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}
			response.TeamLocks[e].Locks = append(response.TeamLocks[e].Locks, &api.Lock{
				LockId:    currentTeamLock.LockID,
				Message:   currentTeamLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentTeamLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentTeamLock.Metadata.CreatedByName,
					Email: currentTeamLock.Metadata.CreatedByEmail,
				},
				CiLink:            currentTeamLock.Metadata.CiLink,
				SuggestedLifetime: currentTeamLock.Metadata.SuggestedLifeTime,
			})
		}

		// Deployments
		deployments, err := o.DBHandler.DBSelectAllLatestDeploymentsForApplication(ctx, transaction, appName)
		if err != nil {
			return nil, fmt.Errorf("could not obtain deployments for app %s: %w", appName, err)
		}

		queuedDeployments, err := o.DBHandler.DBSelectLatestDeploymentAttemptOnAllEnvironments(ctx, transaction, appName)
		if err != nil {
			return nil, err
		}

		// Cache queued versions to check with deployments
		queuedVersions := make(map[types.EnvName]*uint64)
		for _, queuedDeployment := range queuedDeployments {
			if queuedDeployment.ReleaseNumbers.Version != nil {
				parsedInt := *queuedDeployment.ReleaseNumbers.Version
				queuedVersions[queuedDeployment.Env] = &parsedInt
			}
		}
		for envName, currentDeployment := range deployments {

			// Test that deployment's release has the deployment's environment
			deploymentRelease := getReleaseFromVersion(releases, currentDeployment.ReleaseNumbers)
			if deploymentRelease != nil && !slices.Contains(deploymentRelease.Environments, envName) {
				continue
			}

			deployment := &api.Deployment{
				Version:         *currentDeployment.ReleaseNumbers.Version,
				QueuedVersion:   0,
				UndeployVersion: false,
				DeploymentMetaData: &api.Deployment_DeploymentMetaData{
					CiLink:       currentDeployment.Metadata.CiLink,
					DeployAuthor: currentDeployment.Metadata.DeployedByName,
					DeployTime:   currentDeployment.Created.String(),
				},
				Revision: currentDeployment.ReleaseNumbers.Revision,
			}

			queuedVersion, ok := queuedVersions[envName]
			if !ok || queuedVersion == nil {
				deployment.QueuedVersion = 0
			} else {
				deployment.QueuedVersion = *queuedVersion
			}

			if deploymentRelease != nil {
				deployment.UndeployVersion = deploymentRelease.Metadata.UndeployVersion
			}
			response.Deployments[string(envName)] = deployment
		}
		result.UndeploySummary = deriveUndeploySummary(appName, response.Deployments)
		result.Warnings = CalculateWarnings(deployments, appLocks, envGroups)
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	response.Application = resultApp
	return response, nil
}

func getReleaseFromVersion(releases []*db.DBReleaseWithMetaData, releaseNumber types.ReleaseNumbers) *db.DBReleaseWithMetaData {
	for _, curr := range releases {
		if *curr.ReleaseNumbers.Version == *releaseNumber.Version && curr.ReleaseNumbers.Revision == releaseNumber.Revision {
			return curr
		}
	}
	return nil
}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {

	span, ctx := tracer.StartSpanFromContext(ctx, "GetOverview")
	defer span.Finish()

	return o.getOverviewDB(ctx, o.Repository.State())
}

func (o *OverviewServiceServer) GetAllAppLocks(ctx context.Context,
	in *api.GetAllAppLocksRequest) (*api.GetAllAppLocksResponse, error) {

	span, ctx := tracer.StartSpanFromContext(ctx, "GetAllAppLocks")
	defer span.Finish()

	return db.WithTransactionT(o.DBHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetAllAppLocksResponse, error) {
		state := o.Repository.State()

		allAppNames, err := state.GetApplications(ctx, transaction)
		if err != nil {
			return nil, err
		}

		response := api.GetAllAppLocksResponse{
			AllAppLocks: make(map[string]*api.AllAppLocks),
		}

		appLocks, err := o.DBHandler.DBSelectAllActiveAppLocksForSliceApps(ctx, transaction, allAppNames)
		if err != nil {
			return nil, fmt.Errorf("error obtaining application locks: %w", err)
		}
		for _, currentLock := range appLocks {
			e := string(currentLock.Env)
			if _, ok := response.AllAppLocks[e]; !ok {
				response.AllAppLocks[e] = &api.AllAppLocks{AppLocks: make(map[string]*api.Locks)}

			}
			if _, ok := response.AllAppLocks[e].AppLocks[currentLock.App]; !ok {
				response.AllAppLocks[e].AppLocks[currentLock.App] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}

			response.AllAppLocks[e].AppLocks[currentLock.App].Locks = append(response.AllAppLocks[e].AppLocks[currentLock.App].Locks, &api.Lock{
				LockId:    currentLock.LockID,
				Message:   currentLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentLock.Metadata.CreatedByName,
					Email: currentLock.Metadata.CreatedByEmail,
				},
				CiLink:            currentLock.Metadata.CiLink,
				SuggestedLifetime: currentLock.Metadata.SuggestedLifeTime,
			})
		}
		return &response, nil
	})
}

func (o *OverviewServiceServer) GetAllEnvTeamLocks(ctx context.Context,
	in *api.GetAllEnvTeamLocksRequest) (*api.GetAllEnvTeamLocksResponse, error) {

	span, ctx := tracer.StartSpanFromContext(ctx, "GetAllEnvTeamLocks")
	defer span.Finish()

	return db.WithTransactionT(o.DBHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetAllEnvTeamLocksResponse, error) {
		response := api.GetAllEnvTeamLocksResponse{
			AllEnvLocks:  make(map[string]*api.Locks),
			AllTeamLocks: make(map[string]*api.AllTeamLocks),
		}
		allEnvLocks, err := o.DBHandler.DBSelectAllEnvLocksOfAllEnvs(ctx, transaction)
		if err != nil {
			return nil, err
		}
		for envName, envLocks := range allEnvLocks {
			e := string(envName)
			for _, currentLock := range envLocks {
				if _, ok := response.AllEnvLocks[e]; !ok {
					response.AllEnvLocks[e] = &api.Locks{Locks: make([]*api.Lock, 0)}

				}
				response.AllEnvLocks[e].Locks = append(response.AllEnvLocks[e].Locks, &api.Lock{
					LockId:    currentLock.LockID,
					Message:   currentLock.Metadata.Message,
					CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
					CreatedBy: &api.Actor{
						Name:  currentLock.Metadata.CreatedByName,
						Email: currentLock.Metadata.CreatedByEmail,
					},
					CiLink:            currentLock.Metadata.CiLink,
					SuggestedLifetime: currentLock.Metadata.SuggestedLifeTime,
				})
			}
		}
		allTeamLocks, err := o.DBHandler.DBSelectAllTeamLocksOfAllEnvs(ctx, transaction)
		if err != nil {
			return nil, err
		}
		for envName, teams := range allTeamLocks {
			e := string(envName)
			for team, locks := range teams {
				for _, currentLock := range locks {
					if _, ok := response.AllTeamLocks[e]; !ok {
						response.AllTeamLocks[e] = &api.AllTeamLocks{TeamLocks: map[string]*api.Locks{}}
					}
					if _, ok := response.AllTeamLocks[e].TeamLocks[team]; !ok {
						response.AllTeamLocks[e].TeamLocks[team] = &api.Locks{Locks: make([]*api.Lock, 0)}
					}
					response.AllTeamLocks[e].TeamLocks[team].Locks = append(response.AllTeamLocks[e].TeamLocks[team].Locks, &api.Lock{
						LockId:    currentLock.LockID,
						Message:   currentLock.Metadata.Message,
						CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
						CreatedBy: &api.Actor{
							Name:  currentLock.Metadata.CreatedByName,
							Email: currentLock.Metadata.CreatedByEmail,
						},
						CiLink:            currentLock.Metadata.CiLink,
						SuggestedLifetime: currentLock.Metadata.SuggestedLifeTime,
					})
				}
			}
		}
		return &response, nil
	})
}

func (o *OverviewServiceServer) getOverviewDB(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {

	response, err := db.WithTransactionT[api.GetOverviewResponse](s.DBHandler, ctx, db.DefaultNumRetries, false, func(ctx context.Context, transaction *sql.Tx) (*api.GetOverviewResponse, error) {
		var err2 error
		response, err2 := o.getOverview(ctx, s, transaction)
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

func (o *OverviewServiceServer) getOverview(
	ctx context.Context,
	s *repository.State,
	transaction *sql.Tx,
) (*api.GetOverviewResponse, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "CalculateOverview")
	defer span.Finish()
	rev := "0000000000000000000000000000000000000000"
	result := api.GetOverviewResponse{
		Branch:            "",
		ManifestRepoUrl:   "",
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
		LightweightApps:   make([]*api.OverviewApplication, 0),
	}
	result.ManifestRepoUrl = o.RepositoryConfig.URL
	result.Branch = o.RepositoryConfig.Branch
	if envs, err := s.GetAllEnvironmentConfigs(ctx, transaction); err != nil {
		return nil, err
	} else {
		result.EnvironmentGroups = mapper.MapEnvironmentsToGroups(envs)
		for envName, config := range envs {
			var groupName = mapper.DeriveGroupName(config, envName)
			var envInGroup = getEnvironmentInGroup(result.EnvironmentGroups, groupName, envName)

			var argocd api.EnvironmentConfig_ArgoCD
			var argocdConfigs = &api.EnvironmentConfig_ArgoConfigs{
				CommonEnvPrefix: "",
				Configs:         make([]*api.EnvironmentConfig_ArgoCD, 0),
			}
			if config.ArgoCd != nil {
				argocd = *mapper.TransformArgocd(*config.ArgoCd)
			}
			if config.ArgoCdConfigs != nil {
				argocdConfigs = mapper.TransformArgocdConfigs(*config.ArgoCdConfigs)
			}
			env := api.Environment{
				DistanceToUpstream: 0,
				Priority:           api.Priority_PROD,
				Name:               string(envName),
				Config: &api.EnvironmentConfig{
					Upstream:         mapper.TransformUpstream(config.Upstream),
					Argocd:           &argocd,
					EnvironmentGroup: &groupName,
					ArgoConfigs:      argocdConfigs,
				},
			}
			envInGroup.Config = env.Config
		}
	}

	if appTeams, err := s.GetAllApplicationsTeamOwner(ctx, transaction); err != nil {
		return nil, err
	} else {
		for appName, team := range appTeams {
			result.LightweightApps = append(result.LightweightApps, &api.OverviewApplication{Name: appName, Team: team})
		}
	}

	return &result, nil
}

func getEnvironmentInGroup(groups []*api.EnvironmentGroup, groupNameToReturn string, envNameToReturn types.EnvName) *api.Environment {
	for _, currentGroup := range groups {
		if currentGroup.EnvironmentGroupName == groupNameToReturn {
			for _, currentEnv := range currentGroup.Environments {
				if currentEnv.Name == string(envNameToReturn) {
					return currentEnv
				}
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
			loaded := o.response.Load()
			var ov *api.GetOverviewResponse = nil
			if loaded == nil {
				ov, err := o.getOverviewDB(stream.Context(), o.Repository.State())
				if err != nil {
					return fmt.Errorf("could not load overview")
				}
				o.response.Store(ov)
			} else {
				ov = loaded.(*api.GetOverviewResponse)
			}

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

func (o *OverviewServiceServer) StreamChangedApps(in *api.GetChangedAppsRequest,
	stream api.OverviewService_StreamChangedAppsServer) error {
	ch, unsubscribe := o.subscribeChangedApps()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-o.Shutdown:
			return nil
		case changedAppsNames := <-ch:
			if len(changedAppsNames) == 0 { //This only happens when a channel is first triggered, so we send all apps
				allApps, err := o.getAllAppNames(stream.Context())
				if err != nil {
					return err
				}
				changedAppsNames = allApps
			}
			ov := &api.GetChangedAppsResponse{
				ChangedApps: make([]*api.GetAppDetailsResponse, len(changedAppsNames)),
			}
			for idx, appName := range changedAppsNames {
				response, err := o.GetAppDetails(stream.Context(), &api.GetAppDetailsRequest{AppName: appName})
				if err != nil {
					return err
				}
				ov.ChangedApps[idx] = response
			}

			logger.FromContext(stream.Context()).Sugar().Infof("Sending changes apps: '%v'\n", changedAppsNames)
			if err := stream.Send(ov); err != nil {
				logger.FromContext(stream.Context()).Error("error sending changed apps  response:", zap.Error(err), zap.String("changedAppsNames", fmt.Sprintf("%+v", ov)))
				return err
			}

		case <-done:
			return nil
		}
	}
}

func (o *OverviewServiceServer) subscribe() (<-chan struct{}, notify.Unsubscribe) {
	o.overviewStreamingInitFunc.Do(func() {
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

func (o *OverviewServiceServer) subscribeChangedApps() (<-chan notify.ChangedAppNames, notify.Unsubscribe) {
	o.changedAppsStreamingInitFunc.Do(func() {
		ch, unsub := o.Repository.Notify().SubscribeChangesApps()
		// Channels obtained from subscribe are by default triggered
		//
		// This means, we have to wait here until the changedApps are loaded for the first time.
		<-ch
		go func() {
			defer unsub()
			for {
				select {
				case <-o.Shutdown:
					return
				case changedApps := <-ch:
					o.notify.NotifyChangedApps(changedApps)
				}
			}
		}()
	})
	return o.notify.SubscribeChangesApps()
}

func (o *OverviewServiceServer) getAllAppNames(ctx context.Context) ([]string, error) {
	return db.WithTransactionMultipleEntriesT(o.DBHandler, ctx, true, func(ctx context.Context, transaction *sql.Tx) ([]string, error) {
		return o.Repository.State().GetApplications(ctx, transaction)
	})
}

func (o *OverviewServiceServer) update(s *repository.State) {
	r, err := o.getOverviewDB(o.Context, s)
	if err != nil {
		logger.FromContext(o.Context).Error("error getting overview:", zap.Error(err))
		return
	}
	if r == nil {
		logger.FromContext(o.Context).Error("overview is nil")
		return
	}
	o.response.Store(r)
	o.notify.Notify()
}

func deriveUndeploySummary(appName string, deployments map[string]*api.Deployment) api.UndeploySummary {
	var allNormal = true
	var allUndeploy = true
	for _, currentDeployment := range deployments {
		if currentDeployment.UndeployVersion {
			allNormal = false
		} else {
			allUndeploy = false
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

func CalculateWarnings(appDeployments map[types.EnvName]db.Deployment, appLocks []db.ApplicationLock, groups []*api.EnvironmentGroup) []*api.Warning {
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
			if upstreamEnvName == nil {
				// this is already checked on startup and therefore shouldn't happen here
				continue
			}

			envDeployment, ok := appDeployments[types.EnvName(env.Name)]

			if !ok {
				// appName is not deployed here, ignore it
				continue
			}
			versionInEnv := envDeployment.ReleaseNumbers.Version

			upstreamDeployment, ok := appDeployments[types.EnvName(*upstreamEnvName)]
			if !ok {
				// appName is not deployed upstream... that's unusual!
				var warning = api.Warning{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         *versionInEnv,
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
				continue
			}
			versionInUpstreamEnv := upstreamDeployment.ReleaseNumbers.Version
			if *versionInEnv > *versionInUpstreamEnv && len(appLocks) == 0 {
				var warning = api.Warning{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     *versionInUpstreamEnv,
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         *versionInEnv,
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

func (o *OverviewServiceServer) StreamDeploymentHistory(in *api.DeploymentHistoryRequest,
	stream api.OverviewService_StreamDeploymentHistoryServer) error {
	span, ctx, onErr := tracing.StartSpanFromContext(stream.Context(), "StreamDeploymentHistory")
	defer span.Finish()

	startDate := in.StartDate.AsTime()
	endDate := in.EndDate.AsTime().AddDate(0, 0, 1)
	if !endDate.After(startDate) {
		providedEndDate := endDate.AddDate(0, 0, -1)
		return onErr(fmt.Errorf("end date (%s) happens before start date (%s)", providedEndDate.Format(time.DateOnly), startDate.Format(time.DateOnly)))
	}

	err := o.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		count, err := o.DBHandler.DBSelectDeploymentHistoryCount(ctx, transaction, in.Environment, startDate, endDate)
		if err != nil {
			return err
		}

		err = stream.Send(&api.DeploymentHistoryResponse{
			Deployment: "time,app,environment,deployed release version,source repository commit hash,previous release version\n",
			Progress:   uint32(100 / (count + 1)),
		})
		if err != nil {
			return err
		}
		startDateReleases := in.StartDate.AsTime().AddDate(0, -1, 0)

		releases, err := o.DBHandler.DBSelectCommitHashesTimeWindow(ctx, transaction, startDateReleases, endDate)
		if err != nil {
			return fmt.Errorf("error obtaining commit hashes for time window: %v", err)
		}
		query := o.DBHandler.AdaptQuery(`
			SELECT created, releaseversion, appname, envname, revision FROM deployments_history
			WHERE releaseversion IS NOT NULL AND created >= (?) AND created <= (?) AND envname = (?)
			ORDER BY created ASC;
		`)

		//All releases that come in first query
		deploymentRows, err := transaction.QueryContext(ctx, query, startDate, endDate, in.Environment)
		if err != nil {
			return err
		}

		previousReleaseVersions := make(map[string]uint64)
		defer func() { _ = deploymentRows.Close() }()
		//Get all relevant deployments and store its information
		for i := uint64(2); deploymentRows.Next(); i++ {
			var created time.Time
			var releaseVersion uint64
			var revision uint64
			var appName string
			var envName string

			err := deploymentRows.Scan(&created, &releaseVersion, &appName, &envName, &revision)
			if err != nil {
				return err
			}

			previousReleaseVersion, hasPreviousVersion := previousReleaseVersions[appName]
			releaseSourceCommitId, exists := releases[db.ReleaseKey{AppName: appName, ReleaseVersion: releaseVersion, Revision: revision}]
			if !exists {
				// If we couldnt find the release info in the window from [start_data - 1Month, EndDate], we fetch this information directly
				fetchRelease, err := o.DBHandler.DBSelectReleaseByVersion(ctx, transaction, appName, types.ReleaseNumbers{Version: &releaseVersion, Revision: revision}, false)
				if err != nil {
					return err
				}
				if fetchRelease == nil {
					logger.FromContext(ctx).Sugar().Warnf("Could not find information for release %q, skipping deployment of application %q on environment %q!", releaseVersion, appName, envName)
					releaseSourceCommitId = "<no commit hash found>"
				} else {
					releaseSourceCommitId = fetchRelease.Metadata.SourceCommitId
				}
			}
			var line string
			if hasPreviousVersion {
				line = fmt.Sprintf("%s,%s,%s,%d,%s,%d\n", created.Format(time.RFC3339), appName, in.Environment, releaseVersion, releaseSourceCommitId, previousReleaseVersion)
			} else {
				line = fmt.Sprintf("%s,%s,%s,%d,%s,nil\n", created.Format(time.RFC3339), appName, in.Environment, releaseVersion, releaseSourceCommitId)
			}

			err = stream.Send(&api.DeploymentHistoryResponse{
				Deployment: line,
				Progress:   uint32(100 * i / (count + 1)),
			})
			if err != nil {
				return err
			}
			previousReleaseVersions[appName] = releaseVersion
		}

		return nil
	})

	if err != nil {
		return onErr(err)
	}

	return nil
}
