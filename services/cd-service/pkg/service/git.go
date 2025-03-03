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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"go.uber.org/zap"
	"sync"

	"sort"
	"strconv"
	"strings"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	eventmod "github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	billy "github.com/go-git/go-billy/v5"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GitServer struct {
	Config          repository.RepositoryConfig
	OverviewService *OverviewServiceServer
	PageSize        uint64

	shutdown                    <-chan struct{}
	streamGitSyncStatusInitFunc sync.Once
	notify                      notify.Notify
}

func (s *GitServer) GetGitTags(ctx context.Context, _ *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented.  cd-service")
}

func (s *GitServer) GetProductSummary(ctx context.Context, in *api.GetProductSummaryRequest) (*api.GetProductSummaryResponse, error) {
	if in.Environment == nil && in.EnvironmentGroup == nil {
		return nil, fmt.Errorf("Must have an environment or environmentGroup to get the product summary for")
	}
	if in.Environment != nil && in.EnvironmentGroup != nil {
		if *in.Environment != "" && *in.EnvironmentGroup != "" {
			return nil, fmt.Errorf("Can not have both an environment and environmentGroup to get the product summary for")
		}
	}
	if in.ManifestRepoCommitHash == "" {
		return nil, fmt.Errorf("Must have a commit to get the product summary for")
	}
	var summaryFromEnv []api.ProductSummary
	dbHandler := s.Config.DBHandler
	state := s.OverviewService.Repository.State()
	response, err := db.WithTransactionT[api.GetProductSummaryResponse](dbHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetProductSummaryResponse, error) {
		//Translate a manifest repo commit hash into a DB transaction timestamp.
		ts, err := dbHandler.DBReadCommitHashTransactionTimestamp(ctx, transaction, in.ManifestRepoCommitHash)
		if err != nil {
			return nil, fmt.Errorf("unable to get manifest repo timestamp that corresponds to provided commit Hash %v", err)
		}

		if ts == nil {
			return nil, fmt.Errorf("could not find timestamp that corresponds to the given commit hash")
		}
		if in.Environment != nil && *in.Environment != "" {
			//Single environment
			allAppsForEnv, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, *in.Environment, *ts)
			if err != nil {
				return nil, fmt.Errorf("unable to get applications for environment '%s': %v", *in.Environment, err)
			}
			if len(allAppsForEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			for _, currentApp := range allAppsForEnv {
				currentAppDeployments, err := state.GetAllDeploymentsForAppAtTimestamp(ctx, transaction, currentApp, *ts)
				if err != nil {
					return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
				}

				if version, ok := currentAppDeployments[*in.Environment]; ok {
					summaryFromEnv = append(summaryFromEnv, api.ProductSummary{
						CommitId:       "",
						DisplayVersion: "",
						Team:           "",
						App:            currentApp,
						Version:        strconv.FormatInt(version, 10),
						Environment:    *in.Environment,
					})

				}
			}
			if len(summaryFromEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			sort.Slice(summaryFromEnv, func(i, j int) bool {
				a := summaryFromEnv[i].App
				b := summaryFromEnv[j].App
				return a < b
			})
		} else {
			//Environment Group
			var environmentGroups []*api.EnvironmentGroup

			envs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
			if err != nil {
				return nil, fmt.Errorf("could not get environment configs %v", err)
			} else {
				environmentGroups = mapper.MapEnvironmentsToGroups(envs)
			}
			for _, envGroup := range environmentGroups {
				if *in.EnvironmentGroup == envGroup.EnvironmentGroupName {
					for _, env := range envGroup.Environments {
						envName := env.Name
						allAppsForEnv, err := state.GetEnvironmentApplicationsAtTimestamp(ctx, transaction, envName, *ts)
						if err != nil {
							return nil, fmt.Errorf("unable to get all applications for environment '%s': %v", envName, err)
						}
						if len(allAppsForEnv) == 0 {
							return &api.GetProductSummaryResponse{
								ProductSummary: nil,
							}, nil
						}
						for _, currentApp := range allAppsForEnv {

							currentAppDeployments, err := state.GetAllDeploymentsForAppAtTimestamp(ctx, transaction, currentApp, *ts)
							if err != nil {
								return nil, fmt.Errorf("unable to get GetAllDeploymentsForAppAtTimestamp  %v", err)
							}
							if version, ok := currentAppDeployments[envName]; ok {
								summaryFromEnv = append(summaryFromEnv, api.ProductSummary{
									CommitId:       "",
									DisplayVersion: "",
									Team:           "",
									App:            currentApp,
									Version:        strconv.FormatInt(version, 10),
									Environment:    envName,
								})

							}
						}
					}
				}
			}
			if len(summaryFromEnv) == 0 {
				return &api.GetProductSummaryResponse{
					ProductSummary: nil,
				}, nil
			}
			sort.Slice(summaryFromEnv, func(i, j int) bool {
				a := summaryFromEnv[i].App
				b := summaryFromEnv[j].App
				return a < b
			})
		}

		var productVersion []*api.ProductSummary
		for _, row := range summaryFromEnv { //nolint: govet
			v, err := strconv.ParseUint(row.Version, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("could not parse version to integer %s: %v", row.Version, err)
			}
			release, err := dbHandler.DBSelectReleaseByVersionAtTimestamp(ctx, transaction, row.App, v, false, *ts)
			if err != nil {
				return nil, fmt.Errorf("error getting release for version")
			}
			team, err := state.GetApplicationTeamOwnerAtTimestamp(ctx, transaction, row.App, *ts)
			if err != nil {
				return nil, fmt.Errorf("could not find app %s: %v", row.App, err)
			}
			productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: row.Version, CommitId: release.Metadata.SourceCommitId, DisplayVersion: release.Metadata.DisplayVersion, Environment: row.Environment, Team: team})
		}
		return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
	})
	return response, err
}

func (s *GitServer) GetCommitInfo(ctx context.Context, in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented.  cd-service")
}

func (s *GitServer) GetEvents(ctx context.Context, transaction *sql.Tx, fs billy.Filesystem, commitPath string, pageNumber uint64) ([]*api.Event, error) {
	var result []*api.Event
	parts := strings.Split(commitPath, "/")
	commitID := parts[len(parts)-2] + parts[len(parts)-1]

	events, err := s.Config.DBHandler.DBSelectAllEventsForCommit(ctx, transaction, commitID, pageNumber, s.PageSize)
	if err != nil {
		return nil, fmt.Errorf("could not read events from DB: %v", err)
	}
	for _, currEvent := range events {
		ev, err := eventmod.UnMarshallEvent(currEvent.EventType, currEvent.EventJson)
		if err != nil {
			return nil, fmt.Errorf("error processing event from DB: %v", err)
		}
		rawUUID, err := timeuuid.ParseUUID(currEvent.Uuid)
		if err != nil {
			return nil, fmt.Errorf("could not parse UUID: '%s'. Error: %v", currEvent.Uuid, err)
		}

		result = append(result, eventmod.ToProto(rawUUID, ev.EventData))
	}
	return result, nil
}

func (s *GitServer) ReadEvent(ctx context.Context, fs billy.Filesystem, eventPath string, eventId timeuuid.UUID) (*api.Event, error) {
	event, err := eventmod.Read(fs, eventPath)
	if err != nil {
		return nil, err
	}
	return eventmod.ToProto(eventId, event), nil
}

// Implements api.GitServer.GetGitSyncStatus
func (s *GitServer) GetGitSyncStatus(ctx context.Context, _ *api.GetGitSyncStatusRequest) (*api.GetGitSyncStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented.  cd-service")
}

func (s *GitServer) ReadSyncStatuses(ctx context.Context) (*api.GetGitSyncStatusResponse, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "ReadSyncStatuses")
	defer span.Finish()

	dbHandler := s.Config.DBHandler
	response := &api.GetGitSyncStatusResponse{
		AppStatuses: make(map[string]*api.EnvSyncStatus),
	}
	err := dbHandler.WithTransactionR(ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) error {
		unsyncedStatuses, err := dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.UNSYNCED)
		if err != nil {
			return err
		}

		syncFailedStatuses, err := dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.SYNC_FAILED)
		if err != nil {
			return err
		}
		response.AppStatuses = toApiStatuses(append(unsyncedStatuses, syncFailedStatuses...))
		return nil
	})
	return response, onErr(err)
}

func toApiStatuses(statuses []db.GitSyncData) map[string]*api.EnvSyncStatus {
	toFill := make(map[string]*api.EnvSyncStatus)
	for _, currStatus := range statuses {
		if _, exists := toFill[currStatus.AppName]; !exists {
			toFill[currStatus.AppName] = &api.EnvSyncStatus{
				EnvStatus: make(map[string]api.GitSyncStatus),
			}
		}
		var statusToWrite api.GitSyncStatus
		if currStatus.SyncStatus == db.SYNC_FAILED {
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_ERROR
		} else if currStatus.SyncStatus == db.UNSYNCED {
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED
		} else {
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_UNKNOWN
		}

		toFill[currStatus.AppName].EnvStatus[currStatus.EnvName] = statusToWrite
	}
	return toFill
}

func (s *GitServer) subscribeGitSyncStatus() (<-chan struct{}, notify.Unsubscribe) {
	s.streamGitSyncStatusInitFunc.Do(func() {
		ch, unsub := s.OverviewService.Repository.Notify().SubscribeGitSyncStatus()
		// Channels obtained from subscribe are by default triggered
		<-ch
		go func() {
			defer unsub()
			for {
				select {
				case <-s.shutdown:
					return
				case <-ch:
					s.notify.NotifyGitSyncStatus()
				}
			}
		}()
	})
	return s.notify.SubscribeGitSyncStatus()
}

func (s *GitServer) StreamGitSyncStatus(_ *api.GetGitSyncStatusRequest,
	stream api.GitService_StreamGitSyncStatusServer) error {
	span, ctx, onErr := tracing.StartSpanFromContext(stream.Context(), "StreamGitSyncStatus")
	defer span.Finish()
	ch, unsubscribe := s.subscribeGitSyncStatus()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-s.shutdown:
			return nil
		case <-ch:
			response, err := s.ReadSyncStatuses(ctx)
			if err != nil {
				return onErr(err)
			}
			if err := stream.Send(response); err != nil {
				logger.FromContext(ctx).Error("error git sync status response:", zap.Error(err), zap.String("StreamGitSyncStatus", fmt.Sprintf("%+v", response)))
				return onErr(err)
			}
		case <-done:
			return nil
		}
	}
}

func (s *GitServer) RetryFailedEvent(ctx context.Context, _ *api.RetryFailedEventRequest) (*api.RetryFailedEventResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented.  cd-service")
}

func (s *GitServer) SkipEslEvent(ctx context.Context, _ *api.SkipEslEventRequest) (*api.SkipEslEventResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented.  cd-service")
}
