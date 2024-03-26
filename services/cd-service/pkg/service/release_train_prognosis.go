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

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

type ReleaseTrainPrognosisServer struct {
	Repository rp.Repository
	RBACConfig auth.RBACConfig
}

func (s *ReleaseTrainPrognosisServer) GetReleaseTrainPrognosis(ctx context.Context, in *api.ReleaseTrainRequest) (*api.GetReleaseTrainPrognosisResponse, error) {
	t := &rp.ReleaseTrain{
		Authentication: rp.Authentication{
			RBACConfig: s.RBACConfig,
		},
		Target:          in.Target,
		Team:            in.Team,
		CommitHash:      in.CommitHash,
		WriteCommitData: false,
		Repo:            s.Repository,
	}

	prognosis := t.Prognosis(ctx, s.Repository.State())

	if prognosis.Error != nil {
		return nil, prognosis.Error
	}

	ret := &api.GetReleaseTrainPrognosisResponse{
		EnvsPrognoses: make(map[string]*api.ReleaseTrainEnvironmentPrognosis),
	}

	for envName, envPrognosis := range prognosis.EnvironmentPrognoses {
		//exhaustruct:ignore
		retEnvPrognosis := &api.ReleaseTrainEnvironmentPrognosis{}
		switch {
		case envPrognosis.SkipCause != nil:
			retEnvPrognosis.Outcome = envPrognosis.SkipCause
		case envPrognosis.Error != nil:
			// this case should never be reached since an error in the environment prognosis is propagated to the release train prognosis
			return nil, fmt.Errorf("error in an environment release train, environment: %s, error: %w", envName, envPrognosis.Error)
		case envPrognosis.AppsPrognoses != nil:
			retEnvPrognosis.Outcome = &api.ReleaseTrainEnvironmentPrognosis_AppsPrognoses{
				AppsPrognoses: &api.ReleaseTrainEnvironmentPrognosis_AppsPrognosesWrapper{
					Prognoses: make(map[string]*api.ReleaseTrainApplicationPrognosis),
				},
			}
			for appName, appPrognosis := range envPrognosis.AppsPrognoses {
				//exhaustruct:ignore
				retAppPrognosis := &api.ReleaseTrainApplicationPrognosis{}
				if appPrognosis.SkipCause != nil {
					retAppPrognosis.Outcome = appPrognosis.SkipCause
				} else {
					retAppPrognosis.Outcome = &api.ReleaseTrainApplicationPrognosis_DeployedVersion{
						DeployedVersion: appPrognosis.Version,
					}
				}
				retEnvPrognosis.GetAppsPrognoses().Prognoses[appName] = retAppPrognosis
			}
		}
		ret.EnvsPrognoses[envName] = retEnvPrognosis
	}

	return ret, nil
}
