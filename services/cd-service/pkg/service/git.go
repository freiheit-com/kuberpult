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
	"fmt"
	"sort"
	"strconv"
	"strings"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	grpcErrors "github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/go-git/go-billy/v5/util"
)

type GitServer struct {
	Config          repository.RepositoryConfig
	OverviewService *OverviewServiceServer
}

func (s *GitServer) GetGitTags(ctx context.Context, in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, err := repository.GetTags(s.Config, "./repository_tags", ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}

	return &api.GetGitTagsResponse{TagData: tags}, nil
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
	if in.CommitHash == "" {
		return nil, fmt.Errorf("Must have a commit to get the product summary for")
	}
	response, err := s.OverviewService.GetOverview(ctx, &api.GetOverviewRequest{GitRevision: in.CommitHash})
	if err != nil {
		return nil, fmt.Errorf("unable to get overview for %s: %v", in.CommitHash, err)
	}

	var summaryFromEnv []api.ProductSummary
	if in.Environment != nil && *in.Environment != "" {
		for _, group := range response.EnvironmentGroups {
			for _, env := range group.Environments {
				if env.Name == *in.Environment {
					for _, app := range env.Applications {
						summaryFromEnv = append(summaryFromEnv, api.ProductSummary{App: app.Name, Version: strconv.FormatUint(app.Version, 10), Environment: *in.Environment})
					}
				}
			}
		}
		if len(summaryFromEnv) == 0 {
			return &api.GetProductSummaryResponse{}, nil
		}
		sort.Slice(summaryFromEnv, func(i, j int) bool {
			a := summaryFromEnv[i].App
			b := summaryFromEnv[j].App
			return a < b
		})
	} else {
		for _, group := range response.EnvironmentGroups {
			if *in.EnvironmentGroup == group.EnvironmentGroupName {
				for _, env := range group.Environments {
					var singleEnvSummary []api.ProductSummary
					for _, app := range env.Applications {
						singleEnvSummary = append(singleEnvSummary, api.ProductSummary{App: app.Name, Version: strconv.FormatUint(app.Version, 10), Environment: env.Name})
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
	for _, row := range summaryFromEnv {
		for _, app := range response.Applications {
			if row.App == app.Name {
				for _, release := range app.Releases {
					if strconv.FormatUint(release.Version, 10) == row.Version {
						productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: row.Version, CommitId: release.SourceCommitId, DisplayVersion: release.DisplayVersion, Environment: row.Environment, Team: app.Team})
						break
					}
				}
			}
		}
	}
	return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
}

func (s *GitServer) GetCommitInfo(ctx context.Context, in *api.GetCommitInfoRequest) (*api.GetCommitInfoResponse, error) {
	fs := s.OverviewService.Repository.State().Filesystem
	
	commitID := in.CommitHash

	if !valid.SHA1CommitID(commitID) {
		return nil, fmt.Errorf("the provided commit ID %s is not a valid SHA1 hash", commitID)
	}

	commitID = strings.ToLower(commitID)

	commitPath := fs.Join("commits", commitID[:2], commitID[2:])

	if _, err := fs.Stat(commitPath); err != nil {
		return nil, grpcErrors.NotFoundError(ctx, fmt.Errorf("commit %s was not found in the manifest repo", commitID));
	}

	sourceMessagePath := fs.Join(commitPath, "source_message")
	var commitMessage string
	if dat, err := util.ReadFile(fs, sourceMessagePath); err != nil {
		return nil, fmt.Errorf("could not open the source message file at %s, err: %w", sourceMessagePath, err);
	} else {
		commitMessage = string(dat)
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
	sort.Strings(touchedApps);

	return &api.GetCommitInfoResponse{
		CommitMessage: commitMessage,
		TouchedApps: touchedApps,
	}, nil
}