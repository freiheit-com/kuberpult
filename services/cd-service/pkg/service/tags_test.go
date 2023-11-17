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
	"fmt"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestGetProductOverview(t *testing.T) {
	var dev = "dev"
	tcs := []struct {
		Name                   string
		givenEnv               string
		expectedProductSummary []api.ProductSummary
		expectedErr            error
		Setup                  []repository.Transformer
	}{
		{
			Name:        "get Product Overview with no env",
			expectedErr: fmt.Errorf("Must have an environment to get the product summary for"),
		},
		{
			Name:        "get Product Overview with no commitHash",
			givenEnv:    "testing",
			expectedErr: fmt.Errorf("Must have a commit to get the product summary for"),
		},
		{
			Name:     "get Product Overview as expected",
			givenEnv: "development",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "testing25",
					SourceMessage:  "changed something (#678)",
					SourceRepoUrl:  "testing@testing.com/abc",
					DisplayVersion: "v1.0.2",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{{App: "test", Version: "v1.0.2"}},
		},
		{
			Name:     "invalid environment used",
			givenEnv: "staging",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "testing25",
					SourceMessage:  "changed something (#678)",
					SourceRepoUrl:  "testing@testing.com/abc",
					DisplayVersion: "v1.0.2",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedErr: fmt.Errorf("environment development not found"),
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			sv := &TagsServer{OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown}}

			for _, transformer := range tc.Setup {
				repo.Apply(testutil.MakeTestContext(), transformer)
			}
			ov, err := sv.OverviewService.GetOverview(testutil.MakeTestContext(), &api.GetOverviewRequest{})
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
			productSummary, err := sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{CommitHash: ov.GitRevision, Environment: tc.givenEnv})
			if err != nil && tc.expectedErr == nil {
				t.Fatalf("expected no error, but got [%s]: %v", ov.GitRevision, err)
			}
			if err != nil && err.Error() != tc.expectedErr.Error() {
				t.Fatalf("expected the error [%v] but got [%v]", tc.expectedErr, err)
			}
			if productSummary != nil && len(tc.expectedProductSummary) > 0 {
				for iter := range productSummary.ProductSummary {
					if productSummary.ProductSummary[iter].App != tc.expectedProductSummary[iter].App {
						t.Fatalf("expected [%v] for productSummary app name but got [%v]", tc.expectedProductSummary[iter].App, productSummary.ProductSummary[iter].App)
					}
					if productSummary.ProductSummary[iter].Version != tc.expectedProductSummary[iter].Version {
						t.Fatalf("expected [%v] for productSummary app name but got [%v]", tc.expectedProductSummary[iter].Version, productSummary.ProductSummary[iter].Version)
					}
				}
			}

		})
	}
}
