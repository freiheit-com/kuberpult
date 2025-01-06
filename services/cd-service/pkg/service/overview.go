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
	"slices"
	"sort"
	"sync"
	"sync/atomic"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	git "github.com/libgit2/git2go/v34"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if !o.DBHandler.ShouldUseOtherTables() {
		panic("DB")
	}
	resultApp, err := db.WithTransactionT(o.DBHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*api.Application, error) {
		var rels []int64
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
			uintRels[idx] = uint64(r)
		}
		//Does not get the manifest and gets all releases at the same time
		releases, err := o.DBHandler.DBSelectReleasesByVersions(ctx, transaction, appName, uintRels, false)
		if err != nil {
			return nil, err
		}
		for _, currentRelease := range releases {
			var tmp = &repository.Release{
				Version:         currentRelease.ReleaseNumber,
				UndeployVersion: currentRelease.Metadata.UndeployVersion,
				SourceAuthor:    currentRelease.Metadata.SourceAuthor,
				SourceCommitId:  currentRelease.Metadata.SourceCommitId,
				SourceMessage:   currentRelease.Metadata.SourceMessage,
				CreatedAt:       currentRelease.Created,
				DisplayVersion:  currentRelease.Metadata.DisplayVersion,
				IsMinor:         currentRelease.Metadata.IsMinor,
				IsPrepublish:    currentRelease.Metadata.IsPrepublish,
				Environments:    currentRelease.Environments,
			}
			result.Releases = append(result.Releases, tmp.ToProto())
		}
		//Highest to lowest
		sort.Slice(result.Releases, func(i, j int) bool {
			return result.Releases[j].Version < result.Releases[i].Version
		})

		appTeamName, err := o.Repository.State().GetTeamName(ctx, transaction, appName)
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
		envs, err := o.DBHandler.DBSelectEnvironmentsBatch(ctx, transaction, dbAllEnvs.Environments)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve manifests for environments %v from the database, error: %w", dbAllEnvs.Environments, err)
		}

		envMap := make(map[string]db.DBEnvironment)
		envConfigs := make(map[string]config.EnvironmentConfig)
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
			if _, ok := response.AppLocks[currentLock.Env]; !ok {
				response.AppLocks[currentLock.Env] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}
			response.AppLocks[currentLock.Env].Locks = append(response.AppLocks[currentLock.Env].Locks, &api.Lock{
				LockId:    currentLock.LockID,
				Message:   currentLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentLock.Metadata.CreatedByName,
					Email: currentLock.Metadata.CreatedByEmail,
				},
			})
		}

		// Team Locks
		teamLocks, err := o.DBHandler.DBSelectAllActiveTeamLocksForTeam(ctx, transaction, result.Team)
		if err != nil {
			return nil, fmt.Errorf("could not find team locks for app %s: %w", appName, err)
		}
		for _, currentTeamLock := range teamLocks {
			if _, ok := response.TeamLocks[currentTeamLock.Env]; !ok {
				response.TeamLocks[currentTeamLock.Env] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}
			response.TeamLocks[currentTeamLock.Env].Locks = append(response.TeamLocks[currentTeamLock.Env].Locks, &api.Lock{
				LockId:    currentTeamLock.LockID,
				Message:   currentTeamLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentTeamLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentTeamLock.Metadata.CreatedByName,
					Email: currentTeamLock.Metadata.CreatedByEmail,
				},
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
		queuedVersions := make(map[string]*uint64)
		for _, queuedDeployment := range queuedDeployments {
			if queuedDeployment.Version != nil {
				parsedInt := uint64(*queuedDeployment.Version)
				queuedVersions[queuedDeployment.Env] = &parsedInt
			}
		}
		for envName, currentDeployment := range deployments {

			// Test that deployment's release has the deployment's environment
			deploymentRelease := getReleaseFromVersion(releases, uint64(*currentDeployment.Version))
			if deploymentRelease != nil && !slices.Contains(deploymentRelease.Environments, envName) {
				continue
			}

			environment, ok := envMap[envName]
			if !ok {
				logger.FromContext(ctx).Sugar().Warnf("could not obtain environment %s for app %s: %w", envName, appName, err)
				continue
			}
			foundApp := false // only apps that are active on that environment should be returned here
			for _, appInEnv := range environment.Applications {
				if appInEnv == appName {
					foundApp = true
					break
				}
			}
			if foundApp {
				deployment := &api.Deployment{
					Version:         uint64(*currentDeployment.Version),
					QueuedVersion:   0,
					UndeployVersion: false,
					DeploymentMetaData: &api.Deployment_DeploymentMetaData{
						CiLink:       currentDeployment.Metadata.CiLink,
						DeployAuthor: currentDeployment.Metadata.DeployedByName,
						DeployTime:   currentDeployment.Created.String(),
					},
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
				response.Deployments[envName] = deployment
			}
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

func getReleaseFromVersion(releases []*db.DBReleaseWithMetaData, version uint64) *db.DBReleaseWithMetaData {
	for _, curr := range releases {
		if curr.ReleaseNumber == version {
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

	if in.GitRevision != "" {
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
			if _, ok := response.AllAppLocks[currentLock.Env]; !ok {
				response.AllAppLocks[currentLock.Env] = &api.AllAppLocks{AppLocks: make(map[string]*api.Locks)}

			}
			if _, ok := response.AllAppLocks[currentLock.Env].AppLocks[currentLock.App]; !ok {
				response.AllAppLocks[currentLock.Env].AppLocks[currentLock.App] = &api.Locks{Locks: make([]*api.Lock, 0)}
			}

			response.AllAppLocks[currentLock.Env].AppLocks[currentLock.App].Locks = append(response.AllAppLocks[currentLock.Env].AppLocks[currentLock.App].Locks, &api.Lock{
				LockId:    currentLock.LockID,
				Message:   currentLock.Metadata.Message,
				CreatedAt: timestamppb.New(currentLock.Metadata.CreatedAt),
				CreatedBy: &api.Actor{
					Name:  currentLock.Metadata.CreatedByName,
					Email: currentLock.Metadata.CreatedByEmail,
				},
			})
		}
		
		return &response, nil
	})
}

func (o *OverviewServiceServer) getOverviewDB(
	ctx context.Context,
	s *repository.State) (*api.GetOverviewResponse, error) {

	if s.DBHandler.ShouldUseOtherTables() {
		response, err := db.WithTransactionT[api.GetOverviewResponse](s.DBHandler, ctx, db.DefaultNumRetries, false, func(ctx context.Context, transaction *sql.Tx) (*api.GetOverviewResponse, error) {
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
	span, ctx := tracer.StartSpanFromContext(ctx, "CalculateOverview")
	defer span.Finish()
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
		EnvironmentGroups: []*api.EnvironmentGroup{},
		GitRevision:       rev,
		LightweightApps:   make([]*api.OverviewApplication, 0),
	}
	result.ManifestRepoUrl = o.RepositoryConfig.URL
	result.Branch = o.RepositoryConfig.Branch
	err := s.UpdateEnvironmentsInOverview(ctx, transaction, &result)
	if err != nil {
		return nil, err
	}

	if apps, err := s.GetApplications(ctx, transaction); err != nil {
		return nil, err
	} else {
		for _, appName := range apps {

			team, err := s.GetTeamName(ctx, transaction, appName)
			if err != nil {
				return nil, err
			}
			result.LightweightApps = append(result.LightweightApps, &api.OverviewApplication{Name: appName, Team: team})
		}

	}

	return &result, nil
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

func CalculateWarnings(appDeployments map[string]db.Deployment, appLocks []db.ApplicationLock, groups []*api.EnvironmentGroup) []*api.Warning {
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

			envDeployment, ok := appDeployments[env.Name]

			if !ok {
				// appName is not deployed here, ignore it
				continue
			}
			versionInEnv := envDeployment.Version

			upstreamDeployment, ok := appDeployments[*upstreamEnvName]
			if !ok {
				// appName is not deployed upstream... that's unusual!
				var warning = api.Warning{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         uint64(*versionInEnv),
							ThisEnvironment:     env.Name,
						},
					},
				}
				result = append(result, &warning)
				continue
			}
			versionInUpstreamEnv := upstreamDeployment.Version
			if *versionInEnv > *versionInUpstreamEnv && len(appLocks) == 0 {
				var warning = api.Warning{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     uint64(*versionInUpstreamEnv),
							UpstreamEnvironment: *upstreamEnvName,
							ThisVersion:         uint64(*versionInEnv),
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
