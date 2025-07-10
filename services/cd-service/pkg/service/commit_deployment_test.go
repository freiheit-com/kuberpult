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

Copyright freiheit.com*/

package service

import (
	"github.com/freiheit-com/kuberpult/pkg/types"
	"reflect"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
)

func TestGetCommitReleaseNumber(t *testing.T) {
	tcs := []struct {
		name      string
		eventJson []byte
		expected  uint64
	}{
		{
			name:      "ReleaseVersion doesn't exist in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release"}}`),
			expected:  0,
		},
		{
			name:      "ReleaseVersion exists in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release","ReleaseVersion":12}}`),
			expected:  12,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			releaseVersion, err := getCommitReleaseNumber(tc.eventJson)
			if err != nil {
				t.Fatalf("Error getting release version: %v", err)
			}
			if releaseVersion != tc.expected {
				t.Fatalf("Expected %d, got %d", tc.expected, releaseVersion)
			}
		})
	}
}

func TestGetCommitStatus(t *testing.T) {
	tcs := []struct {
		name                string
		releaseNumber       uint64
		allEnvironments     []types.EnvName
		environmentReleases map[types.EnvName]uint64
		expectedStatus      CommitStatus
	}{
		{
			name:                "One environment with newer release",
			releaseNumber:       1,
			allEnvironments:     []types.EnvName{"dev"},
			environmentReleases: map[types.EnvName]uint64{"dev": 2},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_DEPLOYED,
			},
		},
		{
			name:                "One environment with older release",
			releaseNumber:       2,
			allEnvironments:     []types.EnvName{"dev"},
			environmentReleases: map[types.EnvName]uint64{"dev": 1},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "One environment with same release",
			releaseNumber:       1,
			allEnvironments:     []types.EnvName{"dev"},
			environmentReleases: map[types.EnvName]uint64{"dev": 1},
			expectedStatus: CommitStatus{
				"dev": api.CommitDeploymentStatus_DEPLOYED,
			},
		},
		{
			name:                "Multiple environments with different releases",
			releaseNumber:       2,
			allEnvironments:     []types.EnvName{"dev", "staging", "prod"},
			environmentReleases: map[types.EnvName]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_DEPLOYED,
				"staging": api.CommitDeploymentStatus_DEPLOYED,
				"prod":    api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Commit not deployed to all environments",
			releaseNumber:       2,
			allEnvironments:     []types.EnvName{"dev", "staging", "prod", "qa"},
			environmentReleases: map[types.EnvName]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_DEPLOYED,
				"staging": api.CommitDeploymentStatus_DEPLOYED,
				"prod":    api.CommitDeploymentStatus_PENDING,
				"qa":      api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Commit is not deployed anywhere",
			releaseNumber:       2,
			allEnvironments:     []types.EnvName{"dev", "staging", "prod", "qa"},
			environmentReleases: map[types.EnvName]uint64{},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_PENDING,
				"staging": api.CommitDeploymentStatus_PENDING,
				"prod":    api.CommitDeploymentStatus_PENDING,
				"qa":      api.CommitDeploymentStatus_PENDING,
			},
		},
		{
			name:                "Release number is 0",
			releaseNumber:       0,
			allEnvironments:     []types.EnvName{"dev", "staging", "prod", "qa"},
			environmentReleases: map[types.EnvName]uint64{"dev": 3, "staging": 2, "prod": 1},
			expectedStatus: CommitStatus{
				"dev":     api.CommitDeploymentStatus_UNKNOWN,
				"staging": api.CommitDeploymentStatus_UNKNOWN,
				"prod":    api.CommitDeploymentStatus_UNKNOWN,
				"qa":      api.CommitDeploymentStatus_UNKNOWN,
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status := getCommitStatus(tc.releaseNumber, tc.environmentReleases, tc.allEnvironments)
			if !reflect.DeepEqual(status, tc.expectedStatus) {
				t.Fatalf("Expected %v, got %v", tc.expectedStatus, status)
			}
		})
	}
}

func TestGetDeploymentCommitInfo(t *testing.T) {
	devGroup := "development"
	stageGroup := "staging"
	tcs := []struct {
		Name           string
		Setup          []repository.Transformer
		EnvName        string
		AppName        string
		ExpectedResult *api.GetDeploymentCommitInfoResponse
	}{
		{
			Name: "Simple deployment on development",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp",
					Version:     1,
				},
			},
			EnvName: "development",
			AppName: "testapp",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				Author:        "example <example@example.com>",
				CommitMessage: "changed something (#678)",
			},
		},
		{
			Name: "two versions, but taking the first one",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee1",
					SourceAuthor:          "author1",
					SourceMessage:         "message1",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee2",
					SourceAuthor:          "author2",
					SourceMessage:         "message2",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp",
					Version:     2,
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp",
					Version:     1,
				},
			},
			EnvName: "development",
			AppName: "testapp",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbee1",
				Author:        "author1",
				CommitMessage: "message1",
			},
		},
		{
			Name: "two apps",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee1",
					SourceAuthor:          "author1",
					SourceMessage:         "message1",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp1",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee2",
					SourceAuthor:          "author2",
					SourceMessage:         "message2",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp2",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
					},
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp1",
					Version:     1,
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp2",
					Version:     1,
				},
			},
			EnvName: "development",
			AppName: "testapp2",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbee2",
				Author:        "author2",
				CommitMessage: "message2",
			},
		},
		{
			Name: "two environments",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest:      false,
							Environment: "development",
						},
						ArgoCd:           nil,
						EnvironmentGroup: &stageGroup,
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee1",
					SourceAuthor:          "author1",
					SourceMessage:         "message1",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
						"staging":     "manifest",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee2",
					SourceAuthor:          "author2",
					SourceMessage:         "message2",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
						"staging":     "manifest",
					},
				},
				&repository.DeployApplicationVersion{
					Environment: "development",
					Application: "testapp",
					Version:     1,
				},
				&repository.DeployApplicationVersion{
					Environment: "staging",
					Application: "testapp",
					Version:     2,
				},
			},
			EnvName: "staging",
			AppName: "testapp",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbee2",
				Author:        "author2",
				CommitMessage: "message2",
			},
		},
		{
			Name: "no versions",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
			},
			EnvName: "development",
			AppName: "testapp",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "",
				Author:        "",
				CommitMessage: "",
			},
		},
		{
			Name: "no deployments",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: &devGroup,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest:      false,
							Environment: "development",
						},
						ArgoCd:           nil,
						EnvironmentGroup: &stageGroup,
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbee1",
					SourceAuthor:          "author1",
					SourceMessage:         "message1",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "testapp",
					Manifests: map[types.EnvName]string{
						"development": "manifest",
						"staging":     "manifest",
					},
				},
			},
			EnvName: "staging",
			AppName: "testapp",
			ExpectedResult: &api.GetDeploymentCommitInfoResponse{
				CommitId:      "",
				Author:        "",
				CommitMessage: "",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}
			svc := &CommitDeploymentServer{
				DBHandler: repo.State().DBHandler,
			}

			if err := repo.Apply(testutil.MakeTestContext(), tc.Setup...); err != nil {
				t.Fatal(err)
			}

			var ctx = auth.WriteUserToContext(testutil.MakeTestContext(), auth.User{
				Email: "app-email@example.com",
				Name:  "overview tester",
			})

			resp, err := svc.GetDeploymentCommitInfo(ctx, &api.GetDeploymentCommitInfoRequest{
				Environment: tc.EnvName,
				Application: tc.AppName,
			})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.ExpectedResult.Author, resp.Author); diff != "" {
				t.Fatalf("Author mismatch(-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.ExpectedResult.CommitId, resp.CommitId); diff != "" {
				t.Fatalf("Commit Id mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.ExpectedResult.CommitMessage, resp.CommitMessage); diff != "" {
				t.Fatalf("Commit message mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}
