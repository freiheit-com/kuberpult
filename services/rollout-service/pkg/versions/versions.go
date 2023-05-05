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

	"github.com/freiheit-com/kuberpult/services/frontend-service/api"
	"k8s.io/utils/lru"
)

type VersionClient interface {
	GetVersion(ctx context.Context, revision, environment, application string) (uint64, error)
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

func New(client api.OverviewServiceClient) VersionClient {
	result := &versionClient{
		cache: lru.New(20),
    client: client,
	}
	return result
}
