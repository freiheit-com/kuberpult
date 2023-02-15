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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/httperrors"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/types/known/emptypb"
)

type EnvironmentServiceServer struct {
	Repository repository.Repository
}

func (e *EnvironmentServiceServer) CreateEnvironment(
	ctx context.Context,
	in *api.CreateEnvironmentRequest) (*emptypb.Empty, error) {

	upstream := &config.EnvironmentConfigUpstream{}

	if in.Config.Upstream.GetLatest() {
		upstream.Latest = true
	} else if env := in.Config.Upstream.GetEnvironment(); env != "" {
		upstream.Environment = env
	} else {
		upstream = nil
	}

	err := e.Repository.Apply(ctx, &repository.CreateEnvironment{
		Environment: in.Environment,
		Config: config.EnvironmentConfig{
			Upstream: upstream,
			ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Name:                 in.Config.Argocd.Destination.Name,
					Server:               in.Config.Argocd.Destination.Server,
					Namespace:            in.Config.Argocd.Destination.Namespace,
					AppProjectNamespace:  in.Config.Argocd.Destination.AppProjectNamespace,
					ApplicationNamespace: in.Config.Argocd.Destination.ApplicationNamespace,
				},
				SyncWindows:              nil, // FIXME
				ClusterResourceWhitelist: nil, // FIXME
				ApplicationAnnotations:   in.Config.Argocd.ApplicationAnnotations,
				IgnoreDifferences:        nil, // FIXME
				SyncOptions:              in.Config.Argocd.SyncOptions,
			},
			EnvironmentGroup: in.Config.EnvironmentGroup,
		},
	})
	if err != nil {
		return nil, httperrors.InternalError(ctx, err)
	}
	return &emptypb.Empty{}, nil
}
