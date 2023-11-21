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
	"strconv"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type TagsServer struct {
	Config          repository.RepositoryConfig
	OverviewService *OverviewServiceServer
}

func (s *TagsServer) GetGitTags(ctx context.Context, in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, err := repository.GetTags(s.Config, "./repository_tags", ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}

	return &api.GetGitTagsResponse{TagData: tags}, nil
}

func (s *TagsServer) GetProductSummary(ctx context.Context, in *api.GetProductSummaryRequest) (*api.GetProductSummaryResponse, error) {
	if in.Environment == "" {
		return nil, fmt.Errorf("Must have an environment to get the product summary for")
	}
	if in.CommitHash == "" {
		return nil, fmt.Errorf("Must have a commit to get the product summary for")
	}
	response, err := s.OverviewService.GetOverview(ctx, &api.GetOverviewRequest{GitRevision: in.CommitHash})
	if err != nil {
		return nil, fmt.Errorf("unable to get overview for %s: %v", in.CommitHash, err)
	}
	var summaryFromEnv []api.ProductSummary
	for _, group := range response.EnvironmentGroups {
		for _, env := range group.Environments {
			if env.Name == in.Environment {
				for _, app := range env.Applications {
					summaryFromEnv = append(summaryFromEnv, api.ProductSummary{App: app.Name, Version: strconv.Itoa(int(app.Version)), LinkVersion: strconv.Itoa(int(app.Version))})
				}
			}
		}
	}
	if len(summaryFromEnv) == 0 {
		return nil, fmt.Errorf("environment %s not found", in.Environment)
	}

	var productVersion []*api.ProductSummary
	for _, row := range summaryFromEnv {
		for _, app := range response.Applications {
			if row.App == app.Name {
				for _, release := range app.Releases {
					if strconv.Itoa(int(release.Version)) == row.Version {
						if release.DisplayVersion != "" {
							productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: release.DisplayVersion, LinkVersion: row.LinkVersion})
						} else if release.SourceCommitId != "" {
							productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: release.SourceCommitId, LinkVersion: row.LinkVersion})
						} else {
							productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: row.Version, LinkVersion: row.LinkVersion})
						}
						break
					}
				}
			}
		}
	}
	return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
}
