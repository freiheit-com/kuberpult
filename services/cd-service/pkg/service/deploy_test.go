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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"google.golang.org/grpc/status"
)

const (
	envAcceptance   = "acceptance"
	envAcceptanceDE = "acceptance-de"
	envAcceptancePT = "acceptance-pt"
	envProduction   = "production"
	envProductionDE = "production-de"
	envProductionPT = "production-pt"
)

var (
	groupEnvProduction = "prd-group"
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
					testutil.MakeTestContext(),
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
					testutil.MakeTestContext(),
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
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
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
					Target: envProduction,
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'production':

Release Train to 'production' environment:

The release train deployed 3 services from 'acceptance' to 'production'
deployed version 1 of "app1" to "production"

deployed version 1 of "app2" to "production"

deployed version 1 of "app3" to "production"


`,
			shouldSucceed: true,
		},
		{
			Name: "release train for team1 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: envProduction,
					Team:   "team1",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'production':

Release Train to 'production' environment:

The release train deployed 1 services from 'acceptance' to 'production' for team 'team1'
deployed version 1 of "app1" to "production"


`,
			shouldSucceed: true,
		},
		{
			Name: "release train for team2 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: envProduction,
					Team:   "team2",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'production':

Release Train to 'production' environment:

The release train deployed 1 services from 'acceptance' to 'production' for team 'team2'
deployed version 1 of "app2" to "production"


`,
			shouldSucceed: true,
		},
		{
			Name: "Release Train to team3 ( not exists )  ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: envProduction,
					Team:   "team3",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'production':

Release Train to 'production' environment:

The release train deployed 0 services from 'acceptance' to 'production' for team 'team3'

`,
			shouldSucceed: true,
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

			ctx := testutil.MakeTestContext()
			initTransformers := []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
				},
				&repository.CreateEnvironment{
					Environment: envProduction,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance}},
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
			actualMsg := commitMsg[len(commitMsg)-1]

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}

				if d := cmp.Diff(tc.expectedCommitMsg, actualMsg); d != "" {
					t.Fatalf("expected a different message.\n %s", d)
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
func TestGroupReleaseTrainErrors(t *testing.T) {
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
					Target: groupEnvProduction,
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'prd-group':

Release Train to 'production-de' environment:

The release train deployed 3 services from 'acceptance-de' to 'production-de'
deployed version 1 of "app1" to "production-de"

deployed version 1 of "app2" to "production-de"

deployed version 1 of "app3" to "production-de"


Release Train to 'production-pt' environment:

The release train deployed 3 services from 'acceptance-pt' to 'production-pt'
deployed version 1 of "app1" to "production-pt"

deployed version 1 of "app2" to "production-pt"

deployed version 1 of "app3" to "production-pt"


`,
			shouldSucceed: true,
		},
		{
			Name: "release train for team1 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: groupEnvProduction,
					Team:   "team1",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'prd-group':

Release Train to 'production-de' environment:

The release train deployed 1 services from 'acceptance-de' to 'production-de' for team 'team1'
deployed version 1 of "app1" to "production-de"


Release Train to 'production-pt' environment:

The release train deployed 1 services from 'acceptance-pt' to 'production-pt' for team 'team1'
deployed version 1 of "app1" to "production-pt"


`,
			shouldSucceed: true,
		},
		{
			Name: "release train for team2 ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: groupEnvProduction,
					Team:   "team2",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'prd-group':

Release Train to 'production-de' environment:

The release train deployed 1 services from 'acceptance-de' to 'production-de' for team 'team2'
deployed version 1 of "app2" to "production-de"


Release Train to 'production-pt' environment:

The release train deployed 1 services from 'acceptance-pt' to 'production-pt' for team 'team2'
deployed version 1 of "app2" to "production-pt"


`,
			shouldSucceed: true,
		},
		{
			Name: "release train for team3 ( not exists )  ",
			Transformers: []repository.Transformer{
				&repository.ReleaseTrain{
					Target: groupEnvProduction,
					Team:   "team3",
				},
			},
			expectedError: "",
			expectedCommitMsg: `Release Train to environment/environment group 'prd-group':

Release Train to 'production-de' environment:

The release train deployed 0 services from 'acceptance-de' to 'production-de' for team 'team3'

Release Train to 'production-pt' environment:

The release train deployed 0 services from 'acceptance-pt' to 'production-pt' for team 'team3'

`,
			shouldSucceed: true,
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

			ctx := testutil.MakeTestContext()
			initTransformers := []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: envAcceptancePT,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateEnvironment{
					Environment: envAcceptanceDE,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
				},
				&repository.CreateEnvironment{
					Environment: envProductionPT,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptancePT}, EnvironmentGroup: &groupEnvProduction},
				},
				&repository.CreateEnvironment{
					Environment: envProductionDE,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptanceDE}, EnvironmentGroup: &groupEnvProduction},
				},
				&repository.CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envAcceptancePT: "acceptance-pt",
						envAcceptanceDE: "acceptance-de",
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app1",
					Manifests: map[string]string{
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
					Team: "team1",
				},
				&repository.CreateApplicationVersion{
					Application: "app2",
					Manifests: map[string]string{
						envAcceptancePT: "acceptance-pt",
						envAcceptanceDE: "acceptance-de",
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app2",
					Manifests: map[string]string{
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
					Team: "team2",
				},
				&repository.CreateApplicationVersion{
					Application: "app3",
					Manifests: map[string]string{
						envAcceptancePT: "acceptance-pt",
						envAcceptanceDE: "acceptance-de",
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "app3",
					Manifests: map[string]string{
						envProductionPT: "production-pt",
						envProductionDE: "production-de",
					},
				},
			}
			transformers := append(initTransformers, tc.Transformers...)
			commitMsg, _, err := repo.ApplyTransformersInternal(ctx, transformers...)
			actualMsg := commitMsg[len(commitMsg)-1]

			if tc.shouldSucceed {
				if err != nil {
					t.Fatalf("Expected no error: %v", err)
				}
				if d := cmp.Diff(tc.expectedCommitMsg, actualMsg); d != "" {
					t.Fatalf("expected a different message.\n %s", d)
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
