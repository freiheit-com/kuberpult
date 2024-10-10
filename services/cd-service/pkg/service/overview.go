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
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"sync"
	"sync/atomic"

	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type OverviewServiceServer struct {
	Repository       repository.Repository
	RepositoryConfig repository.RepositoryConfig
	Shutdown         <-chan struct{}

	notify   notify.Notify
	Context  context.Context
	init     sync.Once
	response atomic.Value

	DBHandler *db.DBHandler
}

func (o *OverviewServiceServer) GetAppDetails(
	ctx context.Context,
	in *api.GetAppDetailsRequest) (*api.GetAppDetailsResponse, error) {

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
		return nil, fmt.Errorf("the GetAppDetails endpoint is only available when the database is enabled")
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
			rels = retrievedReleasesOfApp.Metadata.Releases
		}

		for _, id := range rels {
			uid := uint64(id)
			// we could optimize this by making one query that does return multiples:
			if rel, err := o.DBHandler.DBSelectReleaseByVersion(ctx, transaction, appName, uid, false); err != nil {
				return nil, err
			} else {
				if rel == nil {
					// ignore
				} else {
					var tmp = &repository.Release{
						Version:         rel.ReleaseNumber,
						UndeployVersion: rel.Metadata.UndeployVersion,
						SourceAuthor:    rel.Metadata.SourceAuthor,
						SourceCommitId:  rel.Metadata.SourceCommitId,
						SourceMessage:   rel.Metadata.SourceMessage,
						CreatedAt:       rel.Created,
						DisplayVersion:  rel.Metadata.DisplayVersion,
						IsMinor:         rel.Metadata.IsMinor,
						IsPrepublish:    rel.Metadata.IsPrepublish,
					}
					release := tmp.ToProto()
					release.Version = uid
					release.UndeployVersion = tmp.UndeployVersion
					result.Releases = append(result.Releases, release)
				}
			}
		}

		if app, err := o.DBHandler.DBSelectApp(ctx, transaction, appName); err != nil {
			return nil, err
		} else {
			if app == nil {
				return nil, fmt.Errorf("could not find app details of app: %s", appName)
			}

			if app.StateChange == db.AppStateChangeDelete {
				return nil, fmt.Errorf("this app has already been deleted: %s", appName)
			}
			result.Team = app.Metadata.Team
		}
		if response == nil {
			return nil, fmt.Errorf("app not found: '%s'", appName)
		}
		envConfigs, err := o.Repository.State().GetAllEnvironmentConfigs(ctx, transaction)
		if err != nil {
			return nil, fmt.Errorf("could not find environments: %w", err)
		}
		envGroups := mapper.MapEnvironmentsToGroups(envConfigs)

		result.Warnings = db.CalculateWarnings(ctx, appName, envGroups)

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
		for envName, currentDeployment := range deployments {
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
			if queuedVersion, err := o.Repository.State().GetQueuedVersion(ctx, transaction, envName, appName); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			} else {
				if queuedVersion == nil {
					deployment.QueuedVersion = 0
				} else {
					deployment.QueuedVersion = *queuedVersion
				}
			}
			if release, err := o.Repository.State().GetApplicationRelease(ctx, transaction, appName, uint64(*currentDeployment.Version)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			} else if release != nil {
				deployment.UndeployVersion = release.UndeployVersion
			}
			response.Deployments[envName] = deployment
		}
		result.UndeploySummary = deriveUndeploySummary(response.Deployments)
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	response.Application = resultApp
	return response, nil

}

func (o *OverviewServiceServer) GetOverview(
	ctx context.Context,
	in *api.GetOverviewRequest) (*api.GetOverviewResponse, error) {
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
		Applications:      map[string]*api.Application{},
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
			err2 := s.UpdateTopLevelAppInOverview(ctx, transaction, appName, &result, false, map[string][]int64{})
			if err2 != nil {
				return nil, err2
			}
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
	r, err := o.getOverviewDB(o.Context, s)
	if err != nil {
		logger.FromContext(o.Context).Error("error getting overview:", zap.Error(err))
		return
	}
	o.response.Store(r)
	o.notify.Notify()
}

func deriveUndeploySummary(deployments map[string]*api.Deployment) api.UndeploySummary {
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
