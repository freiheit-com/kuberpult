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

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TagsServer struct {
	Repository repository.Repository
}

func (s *TagsServer) GetGitTags(ctx context.Context, in *api.GetGitTagsRequest) (*api.GetGitTagsResponse, error) {
	// get repoName from request
	repoName := in.GetRepoName()
	if repoName == "" {
		return nil, status.Error(codes.InvalidArgument, "Must pass a valid repoName to get git tags")
	}
	// get list of git tags
	repo, err := git.InitRepository(repoName, true)
	if err != nil {
		return nil, err
	}
	tags, err := repo.Tags.List()
	if err != nil {
		return nil, err
	}
	var tagsResponse []*api.TagsList
	for _, tag := range tags {

		ref, err := repo.References.Lookup(tag)
		if err != nil {
			return nil, err
		}
		oid := ref.Target()
		commit, err := repo.LookupCommit(oid)
		if err != nil {
			return nil, err
		}
		tagsResponse = append(tagsResponse, &api.TagsList{Tag: tag, CommitId: commit.Id().String()})

	}
	return &api.GetGitTagsResponse{TagList: tagsResponse /* ADD TAGS AND COMMIT HASHES */}, nil
}

var _ api.GitTagsServer = (*TagsServer)(nil)
