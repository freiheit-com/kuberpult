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

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type TagsServer struct {
	Cfg repository.RepositoryConfig
}

func (s *TagsServer) GetGitTags(ctx context.Context, in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	tags, commits, err := repository.GetTags(s.Cfg, "./repository", ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get tags from repository: %v", err)
	}
	var tagsResponse []*api.TagsList
	for i, _ := range tags {
		tagsResponse = append(tagsResponse, &api.TagsList{Tag: tags[i], CommitId: commits[i]})
	}

	return &api.GetGitTagsResponse{TagList: tagsResponse}, nil
}

var _ api.GitTagsServer = (*TagsServer)(nil)
