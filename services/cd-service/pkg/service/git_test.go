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
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	
	"github.com/freiheit-com/kuberpult/pkg/ptr"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	rp "github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/protobuf/proto"
)

func TestGetProductOverview(t *testing.T) {
	tcs := []struct {
		Name                   string
		givenEnv               *string
		givenEnvGroup          *string
		expectedProductSummary []api.ProductSummary
		expectedErr            error
		Setup                  []rp.Transformer
	}{
		{
			Name:        "get Product Overview with no env or envGroup",
			expectedErr: fmt.Errorf("Must have an environment or environmentGroup to get the product summary for"),
		},
		{
			Name:        "get Product Overview with no commitHash",
			givenEnv:    ptr.FromString("testing"),
			expectedErr: fmt.Errorf("Must have a commit to get the product summary for"),
		},
		{
			Name:          "get Product Overview with both env and envGroup",
			givenEnv:      ptr.FromString("testing"),
			givenEnvGroup: ptr.FromString("testingGroup"),
			expectedErr:   fmt.Errorf("Can not have both an environment and environmentGroup to get the product summary for"),
		},
		{
			Name:     "get Product Overview as expected with env",
			givenEnv: ptr.FromString("development"),
			Setup: []rp.Transformer{
				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: ptr.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "testing25",
					SourceMessage:  "changed something (#678)",
					SourceRepoUrl:  "testing@testing.com/abc",
					DisplayVersion: "v1.0.2",
					Team:           "sre-team",
				},
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{{App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "testing25", Team: "sre-team"}},
		},
		{
			Name:     "get Product Overview as expected with env but without team",
			givenEnv: ptr.FromString("development"),
			Setup: []rp.Transformer{
				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: ptr.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
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
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{{App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "testing25", Team: ""}},
		},
		{
			Name:     "invalid environment used",
			givenEnv: ptr.FromString("staging"),
			Setup: []rp.Transformer{
				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: ptr.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
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
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{},
		},
		{
			Name:          "get Product Overview as expected with envGroup",
			givenEnvGroup: ptr.FromString("dev"),
			Setup: []rp.Transformer{
				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: ptr.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "testing25",
					SourceMessage:  "changed something (#678)",
					SourceRepoUrl:  "testing@testing.com/abc",
					DisplayVersion: "v1.0.2",
					Team:           "sre-team",
				},
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{{App: "test", Version: "1", DisplayVersion: "v1.0.2", CommitId: "testing25", Team: "sre-team"}},
		},
		{
			Name:          "invalid envGroup used",
			givenEnvGroup: ptr.FromString("notDev"),
			Setup: []rp.Transformer{
				&rp.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: ptr.FromString("dev"),
					},
				},
				&rp.CreateApplicationVersion{
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
				&rp.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			expectedProductSummary: []api.ProductSummary{},
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
			sv := &GitServer{OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown}}

			for _, transformer := range tc.Setup {
				repo.Apply(testutil.MakeTestContext(), transformer)
			}
			ov, err := sv.OverviewService.GetOverview(testutil.MakeTestContext(), &api.GetOverviewRequest{})
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
			productSummary, err := sv.GetProductSummary(testutil.MakeTestContext(), &api.GetProductSummaryRequest{CommitHash: ov.GitRevision, Environment: tc.givenEnv, EnvironmentGroup: tc.givenEnvGroup})
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

func TestGetCommitInfo(t *testing.T) {
	type TestCase struct {
		name             string
		transformers     []rp.Transformer
		request          *api.GetCommitInfoRequest
		expectedResponse *api.GetCommitInfoResponse
		expectedError    error
	}

	tcs := []TestCase{
		{
			name: "create one commit with one app and get its info",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectedError: nil,
			expectedResponse: &api.GetCommitInfoResponse{
				CommitMessage: "some message",
				TouchedApps: []string{
					"app",
				},
			},
		},
		{
			name: "create one commit with several apps and its info",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app1",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
				},
				&rp.CreateApplicationVersion{
					Application:    "app2",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
				},
				&rp.CreateApplicationVersion{
					Application:    "app3",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			expectedError: nil,
			expectedResponse: &api.GetCommitInfoResponse{
				CommitMessage: "some message",
				TouchedApps: []string{
					"app1",
					"app2",
					"app3",
				},
			},
		},
		{
			name: "create one commit with one app but get the info of a nonexistent commit",
			transformers: []rp.Transformer{
				&rp.CreateApplicationVersion{
					Application:    "app",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "some message",
				},
			},
			request: &api.GetCommitInfoRequest{
				CommitHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			expectedError: status.Error(codes.NotFound, "error: commit bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb was not found in the manifest repo"),
			expectedResponse: nil,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			for _, transformer := range tc.transformers {
				repo.Apply(testutil.MakeTestContext(), transformer)
			}
			sv := &GitServer{OverviewService: &OverviewServiceServer{Repository: repo, Shutdown: shutdown}}

			commitInfo, err := sv.GetCommitInfo(testutil.MakeTestContext(), tc.request)

			if !errors.Is(err, tc.expectedError) {
				t.Fatalf("expected error %v, received error %v", tc.expectedError, err)
			}
			if !proto.Equal(tc.expectedResponse, commitInfo) {
				t.Fatalf("expected response %v, received response %v", tc.expectedResponse, commitInfo)
			}
		})
	}
}
