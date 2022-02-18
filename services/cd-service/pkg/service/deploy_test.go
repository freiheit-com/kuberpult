/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package service

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

const (
	envAcceptance = "acceptance"
	envProduction = "production"
)

func TestDeployService(t *testing.T) {
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, svc *DeployServiceServer)
	}{
		{
			Name: "Deploying a version",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
				},
			},
			Test: func(t *testing.T, svc *DeployServiceServer) {
				_, err := svc.Deploy(
					context.Background(),
					&api.DeployRequest{
						Environment: "production",
						Application: "test",
						Version:     1,
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				{
					version, err := svc.Repository.State().GetEnvironmentApplicationVersion("production", "test")
					if err != nil {
						t.Fatal(err)
					}
					if version == nil {
						t.Errorf("unexpected version: expected 1, actual: %d", version)
					}
					if *version != 1 {
						t.Errorf("unexpected version: expected 1, actual: %d", *version)
					}
				}
			},
		},
		{
			Name: "Deploying a version to a locked environment",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "production",
				},
				&repository.CreateEnvironmentLock{
					Environment: "production",
					LockId:      "a",
					Message:     "b",
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "c",
				},
			},
			Test: func(t *testing.T, svc *DeployServiceServer) {
				_, err := svc.Deploy(
					context.Background(),
					&api.DeployRequest{
						Environment:  "production",
						Application:  "test",
						Version:      1,
						LockBehavior: api.LockBehavior_Fail,
					},
				)
				if err == nil {
					t.Fatal("expected an error but got none")
				}
				stat, ok := status.FromError(err)
				if !ok {
					t.Fatalf("error is not a status error, got: %#v", err)
				}
				details := stat.Details()
				if len(details) == 0 {
					t.Fatalf("error is a status error, but has no details: %s", err.Error())
				}
				lockErr := details[0].(*api.LockedError)
				if _, ok := lockErr.EnvironmentLocks["a"]; !ok {
					t.Errorf("lockErr doesn't contain the environment lock")
				}
				if _, ok := lockErr.EnvironmentApplicationLocks["c"]; !ok {
					t.Errorf("lockErr doesn't contain the application environment lock")
				}
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(context.Background(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &DeployServiceServer{
				Repository: repo,
			}
			tc.Test(t, svc)
		})
	}
}

func TestReleaseTrainErrors(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      []repository.Transformer
		expectedError     string
		expectedCommitMsg string
		shouldSucceed     bool
	}{

		{
			Name: "release train all teams ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Environment: envProduction,
				},
			},
			expectedError:     "",
			expectedCommitMsg: "The release train deployed 3 services from 'acceptance' to 'production'",
			shouldSucceed:     true,
		},
		{
			Name: "release train for team1 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Environment: envProduction,
					Team:        "team1",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "The release train deployed 1 services from 'acceptance' to 'production'",
			shouldSucceed:     true,
		},
		{
			Name: "release train for team2 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Environment: envProduction,
					Team:        "team2",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "The release train deployed 1 services from 'acceptance' to 'production'",
			shouldSucceed:     true,
		},
		{
			Name: "release train for team3 ( not exists )  ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Environment: envProduction,
					Team:        "team3",
				},
			},
			expectedError:     "",
			expectedCommitMsg: "The release train deployed 0 services from 'acceptance' to 'production'",
			shouldSucceed:     true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel()
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			initTransformers := []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&repository.CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&repository.CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
						envProduction: "production",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProduction: "production",
					},
					Team: "team1",
				},
				&repository.CreateApplicationVersion{
					Application: "app2",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
						envProduction: "production",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app2",
					Manifests: map[string]string{
						envProduction: "production",
					},
					Team: "team2",
				},
				&repository.CreateApplicationVersion{
					Application: "app3",
					Manifests: map[string]string{
						envAcceptance: "acceptance",
						envProduction: "production",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app3",
					Manifests: map[string]string{
						envProduction: "production",
					},
				},
			}
			transformers := append(initTransformers, tc.Transformers...)
			commitMsg, _, err := repo.ApplyTransformersInternal(ctx, transformers...)
			// note that we only check the LAST error here:
			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				actualMsg := commitMsg[len(commitMsg)-1]
				Msgs := strings.Split(actualMsg, "\n") // ignoring deploying messages
				if Msgs[0] != tc.expectedCommitMsg {
					t.Fatalf("expected a different message.\nExpected: %q\nGot %q", tc.expectedCommitMsg, Msgs[0])
				}
			} else {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				} else {
					actualMsg := err.Error()
					if actualMsg != tc.expectedError {
						t.Fatalf("expected a different error.\nExpected: %q\nGot %q", tc.expectedError, actualMsg)
					}
				}
			}
		})
	}
}
