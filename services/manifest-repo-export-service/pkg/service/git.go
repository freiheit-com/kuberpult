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
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	eventmod "github.com/freiheit-com/kuberpult/pkg/event"
	grpcErrors "github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"sort"
	"strings"
)

type GitServer struct {
	Config     repository.RepositoryConfig
	Repository repository.Repository
	PageSize   uint64
}

func (s *GitServer) GetProductSummary(_ context.Context, _ *api.GetProductSummaryRequest) (*api.GetProductSummaryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *GitServer) GetGitTags(ctx context.Context, _ *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, err := repository.GetTags(s.Config, "./repository_tags", ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}

	return &api.GetGitTagsResponse{TagData: tags}, nil
}

func (s *GitServer) GetCommitInfo(ctx context.Context, in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	fs := s.Repository.State().Filesystem

	commitIDPrefix, pageNumber := in.CommitHash, in.PageNumber

	commitID, err := findCommitID(ctx, fs, commitIDPrefix)
	if err != nil {
		return nil, err
	}

	commitPath := fs.Join("commits", commitID[:2], commitID[2:])

	sourceMessagePath := fs.Join(commitPath, "source_message")
	var commitMessage string
	if dat, err := util.ReadFile(fs, sourceMessagePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, status.Error(codes.NotFound, "commit info does not exist")
		}
		return nil, fmt.Errorf("could not open the source message file at %s, err: %w", sourceMessagePath, err)
	} else {
		commitMessage = string(dat)
	}

	var previousCommitMessagePath = fs.Join(commitPath, "previousCommit")
	var previousCommitId string
	if data, err := util.ReadFile(fs, previousCommitMessagePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("could not open the previous commit file at %s, err: %w", previousCommitMessagePath, err)
		}
	} else {
		previousCommitId = string(data)
	}

	var nextCommitMessagePath = fs.Join(commitPath, "nextCommit")
	var nextCommitId string
	if data, err := util.ReadFile(fs, nextCommitMessagePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("could not open the next commit file at %s, err: %w", nextCommitMessagePath, err)
		} //If no file exists, there is no next commit
	} else {
		nextCommitId = string(data)
	}

	commitApplicationsDirPath := fs.Join(commitPath, "applications")
	dirs, err := fs.ReadDir(commitApplicationsDirPath)
	if err != nil {
		return nil, fmt.Errorf("could not read the applications directory at %s, error: %w", commitApplicationsDirPath, err)
	}
	touchedApps := make([]string, 0)
	for _, dir := range dirs {
		touchedApps = append(touchedApps, dir.Name())
	}
	sort.Strings(touchedApps)
	var events []*api.Event
	loadMore := false
	events, err = db.WithTransactionMultipleEntriesT(s.Repository.State().DBHandler, ctx, true, func(ctx context.Context, transaction *sql.Tx) ([]*api.Event, error) {
		return s.GetEvents(ctx, transaction, fs, commitPath, pageNumber)
	})
	if len(events) > int(s.PageSize) {
		loadMore = true
		events = events[:len(events)-1]
	}
	if err != nil {
		return nil, err
	}

	return &api.GetCommitInfoResponse{
		CommitHash:         commitID,
		CommitMessage:      commitMessage,
		TouchedApps:        touchedApps,
		Events:             events,
		PreviousCommitHash: previousCommitId,
		NextCommitHash:     nextCommitId,
		LoadMore:           loadMore,
	}, nil
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

func (s *GitServer) ReadEvent(_ context.Context, fs billy.Filesystem, eventPath string, eventId timeuuid.UUID) (*api.Event, error) {
	event, err := eventmod.Read(fs, eventPath)
	if err != nil {
		return nil, err
	}
	return eventmod.ToProto(eventId, event), nil
}

// findCommitID checks if the "commits" directory in the given
// filesystem contains a commit with the given prefix. Returns the
// full hash of the commit, if a unique one can be found. Returns a
// gRPC error that can be directly returned to the client.
func findCommitID(
	ctx context.Context,
	fs billy.Filesystem,
	commitPrefix string,
) (string, error) {
	if !valid.SHA1CommitIDPrefix(commitPrefix) {
		return "", status.Error(codes.InvalidArgument,
			"not a valid commit_hash")
	}
	commitPrefix = strings.ToLower(commitPrefix)
	if len(commitPrefix) == valid.SHA1CommitIDLength {
		// the easy case: the commit has been requested in
		// full length, so we simply check if the file exist
		// and are done.
		commitPath := fs.Join("commits", commitPrefix[:2], commitPrefix[2:])

		if _, err := fs.Stat(commitPath); err != nil {
			return "", grpcErrors.NotFoundError(ctx,
				fmt.Errorf("commit %s was not found in the manifest repo", commitPrefix))
		}

		return commitPrefix, nil
	}
	if len(commitPrefix) < 7 {
		return "", status.Error(codes.InvalidArgument,
			"commit_hash too short (must be at least 7 characters)")
	}
	// the dir we're looking in
	commitDir := fs.Join("commits", commitPrefix[:2])
	files, err := fs.ReadDir(commitDir)
	if err != nil {
		return "", grpcErrors.NotFoundError(ctx,
			fmt.Errorf("commit with prefix %s was not found in the manifest repo", commitPrefix))
	}
	// the prefix of the file we're looking for
	filePrefix := commitPrefix[2:]
	var commitID string
	for _, file := range files {
		fileName := file.Name()
		if !strings.HasPrefix(fileName, filePrefix) {
			continue
		}
		if commitID != "" {
			// another commit has already been found
			return "", status.Error(codes.InvalidArgument,
				"commit_hash is not unique, provide the complete hash (or a longer prefix)")
		}
		commitID = commitPrefix[:2] + fileName
	}
	if commitID == "" {
		return "", grpcErrors.NotFoundError(ctx,
			fmt.Errorf("commit with prefix %s was not found in the manifest repo", commitPrefix))
	}
	return commitID, nil
}

func (s *GitServer) GetGitSyncStatus(ctx context.Context, _ *api.GetGitSyncStatusRequest) (*api.GetGitSyncStatusResponse, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "GetGitSyncStatus")
	defer span.Finish()
	//Query for UNSYNCED
	dbHandler := s.Repository.State().DBHandler
	response := &api.GetGitSyncStatusResponse{
		Unsynced:   make([]*api.EnvApp, 0),
		SyncFailed: make([]*api.EnvApp, 0),
	}
	err := dbHandler.WithTransactionR(ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) error {
		statuses, err := dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.UNSYNCED)
		if err != nil {
			return err
		}
		response.Unsynced = toApiStatuses(statuses, api.GitSyncStatus_UNSYNCED)

		statuses, err = dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.SYNC_FAILED)
		if err != nil {
			return err
		}
		response.SyncFailed = toApiStatuses(statuses, api.GitSyncStatus_SYNC_FAILED)
		return nil
	})
	return response, onErr(err)
}

func toApiStatuses(statuses []db.GitSyncData, status api.GitSyncStatus) []*api.EnvApp {
	toFill := make([]*api.EnvApp, 0)
	for _, currStatus := range statuses {
		toFill = append(toFill, &api.EnvApp{
			ApplicationName: currStatus.AppName,
			EnvironmentName: currStatus.EnvName,
			Status:          status,
		})
	}
	return toFill
}
