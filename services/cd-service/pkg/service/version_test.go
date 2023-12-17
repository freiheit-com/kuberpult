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
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestVersion(t *testing.T) {
	type expectedVersion struct {
		Environment            string
		Application            string
		ExpectedVersion        uint64
		ExpectedDeployedAt     time.Time
		ExpectedSourceCommitId string
	}
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		ExpectedVersions []expectedVersion
	}{
		{
			Name: "simple-tests",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:        "development",
					Application:        "test",
					ExpectedVersion:    1,
					ExpectedDeployedAt: time.Unix(2, 0),
				},
				{
					Environment:     "staging",
					Application:     "test",
					ExpectedVersion: 0,
				},
			},
		},
		{
			Name: "with source commits",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceCommitId: "deadbeef",
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:            "development",
					Application:            "test",
					ExpectedVersion:        1,
					ExpectedDeployedAt:     time.Unix(2, 0),
					ExpectedSourceCommitId: "deadbeef",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			sv := &VersionServiceServer{Repository: repo}

			for i, transformer := range tc.Setup {
				now := time.Unix(int64(i), 0)
				ctx := repository.WithTimeNow(testutil.MakeTestContext(), now)
				err := repo.Apply(ctx, transformer)
				if err != nil {
					t.Fatal(err)
				}
			}
			cid := repo.State().Commit.Id().String()
			for _, ev := range tc.ExpectedVersions {
				res, err := sv.GetVersion(context.Background(), &api.GetVersionRequest{
					GitRevision: cid,
					Application: ev.Application,
					Environment: ev.Environment,
				})
				if err != nil {
					t.Fatal(err)
				}
				if res.Version != ev.ExpectedVersion {
					t.Errorf("got wrong version for %s/%s: expected %d but got %d", ev.Application, ev.Environment, ev.ExpectedVersion, res.Version)
				}
				if ev.ExpectedDeployedAt.IsZero() {
					if res.DeployedAt != nil {
						t.Errorf("got wrong deployed at for %s/%s: expected <nil> but got %q", ev.Application, ev.Environment, res.DeployedAt)
					}
				} else {
					if !res.DeployedAt.AsTime().Equal(ev.ExpectedDeployedAt) {
						t.Errorf("got wrong deployed at for %s/%s: expected %q but got %q", ev.Application, ev.Environment, ev.ExpectedDeployedAt, res.DeployedAt.AsTime())
					}
				}
				if ev.ExpectedSourceCommitId != res.SourceCommitId {
					t.Errorf("go wrong source commit id, expected %q, got %q", ev.ExpectedSourceCommitId, res.SourceCommitId)
				}
			}
		})
	}
}
