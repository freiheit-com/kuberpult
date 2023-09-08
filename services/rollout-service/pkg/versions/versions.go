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

package versions

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"k8s.io/utils/lru"
)

// This is a the user that the rollout service uses to query the versions.
// It is not written to the repository.
var RolloutServiceUser auth.User = auth.User{
	Email: "kuberpult-rollout-service@local",
	Name:  "kuberpult-rollout-service",
}

type VersionClient interface {
	GetVersion(ctx context.Context, revision, environment, application string) (uint64, error)
	ConsumeEvents(ctx context.Context, processor VersionEventProcessor) error
}

type versionClient struct {
	client api.OverviewServiceClient
	cache  *lru.Cache
}

// GetVersion implements VersionClient
func (v *versionClient) GetVersion(ctx context.Context, revision, environment, application string) (uint64, error) {
	var overview *api.GetOverviewResponse
	entry, ok := v.cache.Get(revision)
	if !ok {
		var err error
		ctx = auth.WriteUserToGrpcContext(ctx, RolloutServiceUser)
		overview, err = v.client.GetOverview(ctx, &api.GetOverviewRequest{
			GitRevision: revision,
		})
		if err != nil {
			return 0, fmt.Errorf("requesting overview %q: %w", revision, err)
		}
		v.cache.Add(revision, overview)
	} else {
		overview = entry.(*api.GetOverviewResponse)
	}
	for _, group := range overview.GetEnvironmentGroups() {
		for _, env := range group.GetEnvironments() {
			if env.Name == environment {
				app := env.Applications[application]
				if app == nil {
					return 0, nil
				}
				return app.Version, nil
			}
		}
	}
	return 0, nil
}

type KuberpultEvent struct {
	Environment string
	Application string
	Version     uint64
}

type VersionEventProcessor interface {
	ProcessKuberpultEvent(ctx context.Context, ev KuberpultEvent)
}

type key struct {
	Environment string
	Application string
}

func (v *versionClient) ConsumeEvents(ctx context.Context, processor VersionEventProcessor) error {
	ctx = auth.WriteUserToGrpcContext(ctx, RolloutServiceUser)
outer:
	for {
		client, err := v.client.StreamOverview(ctx, &api.GetOverviewRequest{})
		if err != nil {
			logger.FromContext(ctx).Warn("overview.connect", zap.Error(err))
			continue outer
		}
		versions := map[key]uint64{}
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			overview, err := client.Recv()
			if err != nil {
				grpcErr := grpc.UnwrapGRPCStatus(err)
				if grpcErr != nil {
					if grpcErr.Code() == codes.Canceled {
						return nil
					}

					logger.FromContext(ctx).Warn("overview.stream", zap.Error(err), zap.String("grpc.code", grpcErr.Code().String()), zap.String("grpc.message", grpcErr.Message()))
				} else {
					logger.FromContext(ctx).Warn("overview.stream", zap.Error(err))
				}
				continue
			}
			l := logger.FromContext(ctx).With(zap.String("git.revision", overview.GitRevision))
			v.cache.Add(overview.GitRevision, overview)
			l.Info("overview.get")
			seen := make(map[key]uint64, len(versions))
			for _, envGroup := range overview.EnvironmentGroups {
				for _, env := range envGroup.Environments {
					for _, app := range env.Applications {

						l.Info("version.process", zap.String("application", app.Name), zap.String("environment", env.Name), zap.Uint64("version", app.Version))
						k := key{env.Name, app.Name}
						seen[k] = app.Version
						if versions[k] == app.Version {
							continue
						}
						processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
							Application: app.Name,
							Environment: env.Name,
							Version:     app.Version,
						})
					}
				}
			}
			// Send events with version 0 for deleted applications so that we can react
			// to apps getting deleted.
			for k := range versions {
				if seen[k] == 0 {
					processor.ProcessKuberpultEvent(ctx, KuberpultEvent{
						Application: k.Application,
						Environment: k.Environment,
						Version:     0,
					})
				}
			}
			versions = seen
		}
	}

}

func New(client api.OverviewServiceClient) VersionClient {
	result := &versionClient{
		cache:  lru.New(20),
		client: client,
	}
	return result
}
