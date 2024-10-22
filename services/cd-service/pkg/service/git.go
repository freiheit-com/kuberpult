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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"

	"os"
	"sort"
	"strconv"
	"strings"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	eventmod "github.com/freiheit-com/kuberpult/pkg/event"
	grpcErrors "github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GitServer struct {
	Config          repository.RepositoryConfig
	OverviewService *OverviewServiceServer
	PageSize        uint64
}

func (s *GitServer) GetGitTags(ctx context.Context, _ *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, err := repository.GetTags(s.Config, "./repository_tags", ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}

	return &api.GetGitTagsResponse{TagData: tags}, nil
}

func getProductSummaryForEnv(ctx context.Context, transaction *sql.Tx, state *repository.State, overviewService *OverviewServiceServer, envName string, appDetailsCache map[string]*api.GetAppDetailsResponse) ([]api.ProductSummary, error) {
	var summaryFromEnv []api.ProductSummary
	appsForEnv, err := state.GetEnvironmentApplications(ctx, transaction, envName)
	if err != nil {
		return nil, fmt.Errorf("unable to get apps on environment for %s: %v", envName, err)
	}
	for _, appName := range appsForEnv {
		currentAppDetails, err := overviewService.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: appName})
		if err != nil {
			return nil, fmt.Errorf("unable to get app details for app: '%s': %w", appName, err)
		}
		appDetailsCache[appName] = currentAppDetails
		if _, ok := currentAppDetails.Deployments[envName]; !ok {
			//The call state.GetEnvironmentApplications should guarantee us a deployment, but just in case it doesn't, we warn an error and continue
			logger.FromContext(ctx).Sugar().Warnf("Could not find deployment information for app '%s' on environment '%s'", appName, envName)
			continue
		}
		summaryFromEnv = append(summaryFromEnv, api.ProductSummary{
			CommitId:       "",
			DisplayVersion: "",
			Team:           "",
			App:            appName,
			Version:        strconv.FormatUint(currentAppDetails.Deployments[envName].Version, 10),
			Environment:    envName,
		})
	}
	return summaryFromEnv, nil
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
	if s.Config.DBHandler.ShouldUseOtherTables() {
		dbHandler := s.OverviewService.DBHandler
		state := s.OverviewService.Repository.State()

		response, err := db.WithTransactionT[api.GetProductSummaryResponse](dbHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*api.GetProductSummaryResponse, error) {
			overview, err := s.OverviewService.GetOverview(ctx, &api.GetOverviewRequest{GitRevision: in.ManifestRepoCommitHash})
			if err != nil {
				return nil, fmt.Errorf("unable to get overview for %s: %v", in.ManifestRepoCommitHash, err)
			}
			accessedApps := make(map[string]*api.GetAppDetailsResponse) //store app details we access for later use and avoid calling the endpoint twice for each app
			var summaryFromEnv []api.ProductSummary
			if in.Environment != nil && *in.Environment != "" {
				for _, group := range overview.EnvironmentGroups {
					for _, env := range group.Environments {
						if env.Name == *in.Environment {
							summaryFromEnv, err = getProductSummaryForEnv(ctx, transaction, state, s.OverviewService, env.Name, accessedApps)
							if err != nil {
								return nil, fmt.Errorf("unable to get product summary for environment: '%s'", env.Name)
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
			} else {
				for _, group := range overview.EnvironmentGroups {
					if *in.EnvironmentGroup == group.EnvironmentGroupName {
						for _, env := range group.Environments {
							var singleEnvSummary []api.ProductSummary
							singleEnvSummary, err := getProductSummaryForEnv(ctx, transaction, state, s.OverviewService, env.Name, accessedApps)
							if err != nil {
								return nil, fmt.Errorf("unable to get product summary for environment: '%s': %w", env.Name, err)
							}
							sort.Slice(singleEnvSummary, func(i, j int) bool {
								a := singleEnvSummary[i].App
								b := singleEnvSummary[j].App
								return a < b
							})
							summaryFromEnv = append(summaryFromEnv, singleEnvSummary...)
						}
					}
				}
				if len(summaryFromEnv) == 0 {
					return nil, nil
				}
			}
			var productVersion []*api.ProductSummary

			for _, row := range summaryFromEnv { //nolint: govet
				appsForEnv, err := state.GetEnvironmentApplications(ctx, transaction, row.Environment)
				if err != nil {
					return nil, fmt.Errorf("unable to get environment applications for env: '%s': %w", row.Environment, err)
				}
				for _, app := range appsForEnv {
					if row.App == app {
						currentAppDetails := accessedApps[app]
						if currentAppDetails == nil {
							continue
						}
						for _, release := range currentAppDetails.Application.Releases {
							if strconv.FormatUint(release.Version, 10) == row.Version {
								productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: row.Version, CommitId: release.SourceCommitId, DisplayVersion: release.DisplayVersion, Environment: row.Environment, Team: currentAppDetails.Application.Team})
								break
							}
						}
					}
				}
			}
			return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
		})
		return response, err
	}
	return &api.GetProductSummaryResponse{ProductSummary: nil}, nil
}

func (s *GitServer) GetCommitInfo(ctx context.Context, in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	if !s.Config.WriteCommitData {
		return nil, status.Error(codes.FailedPrecondition, "no written commit info available; set KUBERPULT_GIT_WRITE_COMMIT_DATA=true to enable")
	}

	fs := s.OverviewService.Repository.State().Filesystem

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
	if s.OverviewService.Repository.State().DBHandler.ShouldUseOtherTables() {
		events, err = db.WithTransactionMultipleEntriesT(s.OverviewService.Repository.State().DBHandler, ctx, true, func(ctx context.Context, transaction *sql.Tx) ([]*api.Event, error) {
			return s.GetEvents(ctx, transaction, fs, commitPath, pageNumber)
		})
		if len(events) > int(s.PageSize) {
			loadMore = true
			events = events[:len(events)-1]
		}
	} else {
		events, err = s.GetEvents(ctx, nil, fs, commitPath, pageNumber)
		if len(events) > int(s.PageSize) {
			loadMore = true
		}
		events = events[pageNumber*s.PageSize : min(len(events), int((pageNumber+1)*s.PageSize))]
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

	if s.Config.DBHandler.ShouldUseOtherTables() {
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
	} else {
		allEventsPath := fs.Join(commitPath, "events")
		potentialEventDirs, err := fs.ReadDir(allEventsPath)
		if err != nil {
			return nil, fmt.Errorf("could not read events directory '%s': %v", allEventsPath, err)
		}
		for i := range potentialEventDirs {
			oneEventDir := potentialEventDirs[i]
			if oneEventDir.IsDir() {
				fileName := oneEventDir.Name()
				rawUUID, err := timeuuid.ParseUUID(fileName)
				if err != nil {
					return nil, fmt.Errorf("could not read event directory '%s' not a UUID: %v", fs.Join(allEventsPath, fileName), err)
				}

				var event *api.Event
				event, err = s.ReadEvent(ctx, fs, fs.Join(allEventsPath, fileName), rawUUID)
				if err != nil {
					return nil, fmt.Errorf("could not read events %v", err)
				}
				result = append(result, event)
			}
		}
		// NOTE: We only sort when using the manifest repo because the db already sorts
		sort.Slice(result, func(i, j int) bool {
			return result[i].CreatedAt.AsTime().UnixNano() < result[j].CreatedAt.AsTime().UnixNano()
		})
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
