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
	"strings"
	"sync"

	billy "github.com/go-git/go-billy/v5"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/db"
	eventmod "github.com/freiheit-com/kuberpult/pkg/event"
	grpcErrors "github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logging"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

type GitServer struct {
	Config     repository.RepositoryConfig
	Repository repository.Repository
	PageSize   uint64

	shutdown                    <-chan struct{}
	streamGitSyncStatusInitFunc sync.Once
	notify                      notify.Notify
	DBHandler                   *db.DBHandler
	RBACConfig                  auth.RBACConfig
}

func (s *GitServer) checkUserPermissions(ctx context.Context, permission string) error {
	if !s.RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf("checkUserPermissions: user not found: %v", err)
	}
	return auth.CheckUserPermissions(s.RBACConfig, user, "*", "", "*", "*", permission)
}

func (s *GitServer) GetGitTags(ctx context.Context, _ *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	var tags []*api.TagData
	if s.Config.TagsPath == "" {
		return nil, fmt.Errorf("tagsPath must not be empty")
	}
	tags, err := repository.GetTags(ctx, s.DBHandler, s.Config, s.Config.TagsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}
	return &api.GetGitTagsResponse{TagData: tags}, nil
}

func (s *GitServer) GetCommitInfo(ctx context.Context, in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	dbHandler := s.Repository.State().DBHandler

	commitInfo, err := db.WithTransactionT(dbHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetCommitInfoResponse, error) {
		commitPrefix, pageNumber := in.CommitHash, in.PageNumber

		err := validateCommitPrefix(commitPrefix)
		if err != nil {
			return nil, err
		}

		releases, err := dbHandler.DBSelectAllReleasesBySourceCommit(ctx, transaction, commitPrefix, true)
		if err != nil {
			return nil, fmt.Errorf("could not read release with SourceCommitId prefix %s from DB: %v", commitPrefix, err)
		}
		if len(releases) == 0 {
			return nil, grpcErrors.NotFoundError(ctx,
				fmt.Errorf("SourceCommitId with prefix %s was not found in the DB", commitPrefix))
		}

		commitID := releases[0].Metadata.SourceCommitId
		sourceMessage := releases[0].Metadata.SourceMessage
		prevCommitID := releases[0].Metadata.PreviousCommitId

		var touchedApps []string
		for _, release := range releases {
			touchedApps = append(touchedApps, string(release.App))
		}

		var nextCommitID string
		nextRelease, err := dbHandler.DBSelectLatestReleaseByPreviousCommit(ctx, transaction, commitID, true)
		if err != nil {
			return nil, fmt.Errorf("could not read release with PreviousCommitId %s from DB: %v", commitID, err)
		}
		if nextRelease != nil {
			nextCommitID = nextRelease.Metadata.SourceCommitId
		}

		loadMore := false
		events, err := s.GetEvents(ctx, transaction, commitID, pageNumber)
		if err != nil {
			return nil, err
		}
		if len(events) > int(s.PageSize) {
			loadMore = true
			events = events[:len(events)-1]
		}

		return &api.GetCommitInfoResponse{
			CommitHash:         commitID,
			CommitMessage:      sourceMessage,
			TouchedApps:        touchedApps,
			Events:             events,
			PreviousCommitHash: prevCommitID,
			NextCommitHash:     nextCommitID,
			LoadMore:           loadMore,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return commitInfo, nil
}

func (s *GitServer) GetEvents(ctx context.Context, transaction *sql.Tx, commitID string, pageNumber uint64) ([]*api.Event, error) {
	var result []*api.Event

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

func (s *GitServer) ReadEvent(_ context.Context, fs billy.Filesystem, eventPath string, eventId timeuuid.UUID) (*api.Event, error) {
	event, err := eventmod.Read(fs, eventPath)
	if err != nil {
		return nil, err
	}
	return eventmod.ToProto(eventId, event), nil
}

func validateCommitPrefix(commitPrefix string) error {
	if !valid.SHA1CommitIDPrefix(commitPrefix) {
		return status.Error(codes.InvalidArgument, "not a valid commit_hash")
	}

	commitPrefix = strings.ToLower(commitPrefix)
	if len(commitPrefix) < 7 {
		return status.Error(codes.InvalidArgument, "commit_hash too short (must be at least 7 characters)")
	}

	return nil
}

func (s *GitServer) GetGitSyncStatus(ctx context.Context, _ *api.GetGitSyncStatusRequest) (_ *api.GetGitSyncStatusResponse, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "GetGitSyncStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	dbHandler := s.Repository.State().DBHandler
	response := &api.GetGitSyncStatusResponse{
		AppStatuses: make(map[string]*api.EnvSyncStatus),
	}
	err = dbHandler.WithTransactionR(ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) error {
		delaySecs, delayEvents, err := dbHandler.GetCurrentDelays(ctx, transaction)
		if err != nil {
			return fmt.Errorf("GetCurrentDelays: %v", err)
		}
		response.ProcessDelaySeconds = delaySecs
		response.ProcessDelayEvents = delayEvents
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
	return response, err
}

func toApiStatuses(statuses []db.GitSyncData) map[string]*api.EnvSyncStatus {
	toFill := make(map[string]*api.EnvSyncStatus)
	for _, currStatus := range statuses {
		if _, exists := toFill[string(currStatus.AppName)]; !exists {
			toFill[string(currStatus.AppName)] = &api.EnvSyncStatus{
				EnvStatus: make(map[string]api.GitSyncStatus),
			}
		}
		var statusToWrite api.GitSyncStatus
		switch currStatus.SyncStatus {
		case db.SYNC_FAILED:
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_ERROR
		case db.UNSYNCED:
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_UNSYNCED
		default:
			statusToWrite = api.GitSyncStatus_GIT_SYNC_STATUS_UNKNOWN
		}

		toFill[string(currStatus.AppName)].EnvStatus[string(currStatus.EnvName)] = statusToWrite
	}
	return toFill
}

func (s *GitServer) subscribeGitSyncStatus() (<-chan struct{}, notify.Unsubscribe) {
	s.streamGitSyncStatusInitFunc.Do(func() {
		ch, unsub := s.Repository.Notify().Subscribe()
		// Channels obtained from subscribe are by default triggered
		<-ch
		go func() {
			defer unsub()
			for {
				select {
				case <-s.shutdown:
					return
				case <-ch:

					s.notify.Notify()
				}
			}
		}()
	})
	return s.notify.Subscribe()
}

func (s *GitServer) StreamGitSyncStatus(in *api.GetGitSyncStatusRequest,
	stream api.ManifestExportGitService_StreamGitSyncStatusServer) (err error) {
	span, ctx := tracer.StartSpanFromContext(stream.Context(), "StreamGitSyncStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	ch, unsubscribe := s.subscribeGitSyncStatus()
	defer unsubscribe()
	done := stream.Context().Done()
	for {
		select {
		case <-s.shutdown:
			return nil
		case <-ch:
			response, err := s.GetGitSyncStatus(ctx, in)
			if err != nil {
				return err
			}
			if err := stream.Send(response); err != nil {
				logging.Error(ctx, "error git sync status response.", zap.Error(err), zap.String("StreamGitSyncStatus", fmt.Sprintf("%+v", response)))
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (s *GitServer) RetryFailedEvent(ctx context.Context, in *api.RetryFailedEventRequest) (_ *api.RetryFailedEventResponse, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RetryFailedEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	err = s.checkUserPermissions(ctx, auth.PermissionRetryFailedEvent)
	if err != nil {
		return nil, err
	}

	dbHandler := s.Repository.State().DBHandler
	response := &api.RetryFailedEventResponse{}
	err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
		failedEvent, err := dbHandler.DBReadEslFailedEventFromEslVersion(ctx, transaction, in.Eslversion)
		if err != nil {
			return err
		}
		if failedEvent == nil {
			return fmt.Errorf("couldn't find failed event with eslVersion: %d", in.Eslversion)
		}
		err = dbHandler.DBWriteEslEventWithJson(ctx, transaction, failedEvent.EventType, failedEvent.EventJson)
		if err != nil {
			return err
		}
		internal, err := dbHandler.DBReadEslEventInternal(ctx, transaction, false)
		if err != nil {
			return err
		}
		err = dbHandler.DBDeleteFailedEslEvent(ctx, transaction, failedEvent)
		if err != nil {
			return err
		}
		err = dbHandler.DBBulkUpdateAllApps(ctx, transaction, db.TransformerID(internal.EslVersion), db.TransformerID(failedEvent.TransformerEslVersion), db.UNSYNCED)
		if err != nil {
			return err
		}
		err = dbHandler.DBBulkUpdateAllDeployments(ctx, transaction, db.TransformerID(internal.EslVersion), db.TransformerID(failedEvent.TransformerEslVersion))
		if err != nil {
			return err
		}
		err = repository.MeasureGitSyncStatus(ctx, s.Config.DDMetrics, dbHandler)
		if err != nil {
			logging.Info(ctx, "Could not send git sync status metrics to datadog", zap.Error(err))
		}

		return nil
	})
	s.Repository.Notify().Notify() //Notify sync statuses have changed
	return response, err
}

func (s *GitServer) SkipEslEvent(ctx context.Context, in *api.SkipEslEventRequest) (_ *api.SkipEslEventResponse, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "SkipEslEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	err = s.checkUserPermissions(ctx, auth.PermissionSkipEslEvent)
	if err != nil {
		return &api.SkipEslEventResponse{}, err
	}

	dbHandler := s.Repository.State().DBHandler

	err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
		failedEvent, err := dbHandler.DBReadEslFailedEventFromEslVersion(ctx, transaction, in.EventEslVersion)
		if err != nil {
			return err
		}
		if failedEvent == nil {
			return fmt.Errorf("couldn't find failed event with eslVersion: %d", in.EventEslVersion)
		}
		return dbHandler.DBSkipFailedEslEvent(ctx, transaction, db.TransformerID(in.EventEslVersion))
	})
	return &api.SkipEslEventResponse{}, err
}
