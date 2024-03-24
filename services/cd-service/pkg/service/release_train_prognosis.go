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

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type ReleaseTrainPrognosisServer struct {
	Repository rp.Repository
	RBACConfig auth.RBACConfig
}

func (s *ReleaseTrainPrognosisServer) GetReleaseTrainPrognosis(ctx context.Context, in *api.ReleaseTrainRequest) (*api.GetReleaseTrainPrognosisResponse, error) {
	s.Repository.ApplyTransformersInternal(ctx, []rp.Transformer{
		&rp.ReleaseTrain{
			Authentication: rp.Authentication{
				RBACConfig: s.RBACConfig,
			},
			Target:          in.Target,
			Team:            in.Team,
			CommitHash:      in.CommitHash,
			WriteCommitData: false,
			Repo:            s.Repository,
		},
	}...)
	
	
	return &api.GetReleaseTrainPrognosisResponse{
		Deployment: []string{
			"placeholder",
		},
	}, nil
}
