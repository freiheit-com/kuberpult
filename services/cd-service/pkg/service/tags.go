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
	Cfg         repository.RepositoryConfig
	OverviewSrv *OverviewServiceServer
}

func (s *TagsServer) GetGitTags(ctx context.Context, in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, err := repository.GetTags(s.Cfg, "./repository_tags", ctx)
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
		return nil, fmt.Errorf("Must have an a commit to get the product summary for")
	}
	response, err := s.OverviewSrv.GetOverview(ctx, &api.GetOverviewRequest{GitRevision: in.CommitHash})
	if err != nil {
		return nil, fmt.Errorf("unable to get overview for %s: %v", in.CommitHash, err)
	}
	var summaryFromEnv []api.ProductSummary
	for _, group := range response.EnvironmentGroups {
		for _, env := range group.Environments {
			if env.Name == in.Environment {
				for _, app := range env.Applications {
					summaryFromEnv = append(summaryFromEnv, api.ProductSummary{App: app.Name, Version: strconv.Itoa(int(app.Version))})
				}
			}
		}
	}
	if len(summaryFromEnv) == 0 {
		return nil, fmt.Errorf("environment %s did not match the existing environments", in.Environment)
	}

	var productVersion []*api.ProductSummary
	for _, row := range summaryFromEnv {
		for _, app := range response.Applications {
			if row.App == app.Name {
				if app.Releases[0].DisplayVersion != "" {
					productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: app.Releases[0].DisplayVersion})
				} else if app.Releases[0].SourceCommitId != "" {
					productVersion = append(productVersion, &api.ProductSummary{App: row.App, Version: app.Releases[0].SourceCommitId})
				} else {
					productVersion = append(productVersion, &row)
				}
			}
		}
	}
	return &api.GetProductSummaryResponse{ProductSummary: productVersion}, nil
}
