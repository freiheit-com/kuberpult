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
	"context"
	"database/sql"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"sort"
	"sync"
	"testing"
	"time"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockOverviewService_StreamOverviewServer struct {
	grpc.ServerStream
	Results chan *api.GetOverviewResponse
	Ctx     context.Context
}

func (m *mockOverviewService_StreamOverviewServer) Send(msg *api.GetOverviewResponse) error {
	m.Results <- msg
	return nil
}

func (m *mockOverviewService_StreamOverviewServer) Context() context.Context {
	return m.Ctx
}

type mockOverviewService_DeploymentHistoryServer struct {
	grpc.ServerStream
	Results chan *api.DeploymentHistoryResponse
	Ctx     context.Context
}

func (m *mockOverviewService_DeploymentHistoryServer) Send(msg *api.DeploymentHistoryResponse) error {
	m.Results <- msg
	return nil
}

func (m *mockOverviewService_DeploymentHistoryServer) Context() context.Context {
	return m.Ctx
}

func TestOverviewAndAppDetails(t *testing.T) {
	var dev types.EnvName = "dev"
	var development types.EnvName = "development"
	var staging types.EnvName = "staging"
	var prod types.EnvName = "production"
	var upstreamLatest = true
	tcs := []struct {
		Name               string
		Setup              []repository.Transformer
		Test               func(t *testing.T, svc *OverviewServiceServer)
		ExpectedOverview   *api.GetOverviewResponse
		AppNamesToCheck    []string
		ExpectedAppDetails map[string]*api.GetAppDetailsResponse //appName -> appDetails
	}{
		{
			Name: "A simple overview works",
			AppNamesToCheck: []string{
				"test",
				"test-with-team",
				"test-with-incorrect-pr-number",
				"test-with-only-pr-number",
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						ArgoCdConfigs:    testutil.MakeArgoCDConfigs("aa", "dev", 2),
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "development",
						},
					},
				},
				&repository.CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Revision:    0,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-team",
					Version:     2,
					Revision:    0,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					Team:           "test-team",
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "test with team version 2",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-incorrect-pr-number",
					Version:     3,
					Revision:    0,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-only-pr-number",
					Version:     4,
					Revision:    0,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "(#678)",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
				&repository.DeployApplicationVersion{
					Application: "test-with-team",
					Environment: "development",
					Version:     2,
				},
				&repository.DeployApplicationVersion{
					Application: "test-with-incorrect-pr-number",
					Environment: prod,
					Version:     3,
				},
				&repository.CreateEnvironmentLock{
					Environment: "development",
					LockId:      "manual",
					Message:     "please",
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "production",
					Application: "test",
					LockId:      "manual",
					Message:     "no",
				},
				&repository.CreateEnvironmentTeamLock{
					Environment: "development",
					Team:        "test-team",
					LockId:      "manual-team-lock",
					Message:     "team lock message",
				},
			},
			ExpectedAppDetails: map[string]*api.GetAppDetailsResponse{
				"test": {
					Application: &api.Application{
						Name:          "test",
						SourceRepoUrl: "",
						Releases: []*api.Release{
							{
								Version:        1,
								PrNumber:       "678",
								SourceAuthor:   "example <example@example.com>",
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceMessage:  "changed something (#678)",
								Environments:   []string{"development"},
							},
						},
						Warnings: []*api.Warning{},
					},
					Deployments: map[string]*api.Deployment{
						"development": {
							Version:         1,
							QueuedVersion:   0,
							UndeployVersion: false,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{
								DeployAuthor: "test tester",
							},
						},
					},
					TeamLocks: map[string]*api.Locks{},
					AppLocks: map[string]*api.Locks{
						string(prod): {
							Locks: []*api.Lock{
								{
									Message: "no",
									LockId:  "manual",
									CreatedBy: &api.Actor{
										Name:  "test tester",
										Email: "testmail@example.com",
									},
								},
							},
						},
					},
				},
				"test-with-team": {
					Application: &api.Application{
						Name: "test-with-team",
						Team: "test-team",
						Releases: []*api.Release{
							{
								Version:        2,
								SourceAuthor:   "example <example@example.com>",
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceMessage:  "test with team version 2",
								Environments:   []string{"development"},
							},
						},
						Warnings: []*api.Warning{},
					},
					Deployments: map[string]*api.Deployment{
						"development": {
							Version:         2,
							QueuedVersion:   0,
							UndeployVersion: false,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{
								DeployAuthor: "test tester",
							},
						},
					},
					TeamLocks: map[string]*api.Locks{
						"development": {
							Locks: []*api.Lock{
								{
									Message: "team lock message",
									LockId:  "manual-team-lock",
									CreatedBy: &api.Actor{
										Name:  "test tester",
										Email: "testmail@example.com",
									},
								},
							},
						},
					},
					AppLocks: map[string]*api.Locks{},
				},
				"test-with-incorrect-pr-number": {
					Application: &api.Application{
						Name: "test-with-incorrect-pr-number",
						Releases: []*api.Release{
							{
								Version:        3,
								SourceAuthor:   "example <example@example.com>",
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceMessage:  "changed something (#678",
								Environments:   []string{"development"},
							},
						},
						Warnings: []*api.Warning{
							{
								WarningType: &api.Warning_UpstreamNotDeployed{
									UpstreamNotDeployed: &api.UpstreamNotDeployed{
										UpstreamEnvironment: "staging",
										ThisVersion:         3,
										ThisEnvironment:     "production",
									},
								},
							},
						},
					},
					Deployments: map[string]*api.Deployment{
						"development": {
							Version:         3,
							QueuedVersion:   0,
							UndeployVersion: false,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{
								DeployAuthor: "test tester",
							},
						},
					},
					AppLocks:  map[string]*api.Locks{},
					TeamLocks: map[string]*api.Locks{},
				},
				"test-with-only-pr-number": {
					Application: &api.Application{
						Name: "test-with-only-pr-number",
						Releases: []*api.Release{
							{
								Version:        4,
								PrNumber:       "678",
								SourceAuthor:   "example <example@example.com>",
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceMessage:  "(#678)",
								Environments:   []string{"development"},
							},
						},
						Warnings: []*api.Warning{},
					},
					Deployments: map[string]*api.Deployment{
						"development": {
							Version:         4,
							QueuedVersion:   0,
							UndeployVersion: false,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{
								DeployAuthor: "test tester",
							},
						},
					},
					AppLocks:  map[string]*api.Locks{},
					TeamLocks: map[string]*api.Locks{},
				},
			},
			ExpectedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: string(development),
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									ArgoConfigs:      transformArgoCdConfigsToApi(testutil.MakeArgoCDConfigs("aa", "dev", 2)),
									EnvironmentGroup: types.StringPtr(dev),
								},
								Priority: api.Priority_UPSTREAM,
							},
						},
						Priority: api.Priority_UPSTREAM,
					},
					{
						EnvironmentGroupName: string(staging),
						Environments: []*api.Environment{
							{
								Name: string(staging),
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: types.StringPtr(development),
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									ArgoConfigs:      &api.EnvironmentConfig_ArgoConfigs{},
									EnvironmentGroup: types.StringPtr(staging),
								},
								DistanceToUpstream: 1,
								Priority:           api.Priority_PRE_PROD,
							},
						},
						Priority:           api.Priority_PRE_PROD,
						DistanceToUpstream: 1,
					},
					{
						EnvironmentGroupName: string(prod),
						Environments: []*api.Environment{
							{
								Name: string(prod),
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: types.StringPtr(staging),
									},
									Argocd:           &api.EnvironmentConfig_ArgoCD{},
									ArgoConfigs:      &api.EnvironmentConfig_ArgoConfigs{},
									EnvironmentGroup: types.StringPtr(prod),
								},
								DistanceToUpstream: 2,
								Priority:           api.Priority_PROD,
							},
						},
						Priority:           api.Priority_PROD,
						DistanceToUpstream: 2,
					},
				},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
						Team: "",
					},
					{
						Name: "test-with-incorrect-pr-number",
						Team: "",
					},
					{
						Name: "test-with-only-pr-number",
						Team: "",
					},
					{
						Name: "test-with-team",
						Team: "test-team",
					},
				},
				GitRevision: "0000000000000000000000000000000000000000",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository

			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}

			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			ctx := testutil.MakeTestContext()
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    ctx,
			}
			ov, err := svc.GetOverview(ctx, &api.GetOverviewRequest{GitRevision: ""})
			if err != nil {
				t.Error(err)
			}
			sort.Slice(ov.LightweightApps, func(i, j int) bool {
				return ov.LightweightApps[i].Name < ov.LightweightApps[j].Name
			})
			if diff := cmp.Diff(tc.ExpectedOverview, ov, protocmp.Transform(), getOverviewIgnoredTypes(), protocmp.IgnoreFields(&api.Lock{}, "created_at")); diff != "" {
				t.Errorf("overview missmatch (-want, +got): %s", diff)
			}
			for _, appName := range tc.AppNamesToCheck {
				appDetails, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: appName})
				if err != nil {
					t.Error(err)
				}

				if diff := cmp.Diff(tc.ExpectedAppDetails[appName], appDetails, getAppDetailsIgnoredTypes(), cmpopts.IgnoreFields(api.Release{}, "CreatedAt"), cmpopts.IgnoreFields(api.Deployment_DeploymentMetaData{}, "DeployTime"), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt")); diff != "" {
					t.Errorf("response missmatch (-want, +got): %s", diff)
				}
			}
		})
	}
}
func TestOverviewService(t *testing.T) {
	var dev types.EnvName = "dev"
	tcs := []struct {
		Name               string
		Setup              []repository.Transformer
		Test               func(t *testing.T, svc *OverviewServiceServer)
		DB                 bool
		ExpectedAppDetails map[string]*api.GetAppDetailsResponse //appName -> appDetails
	}{
		{
			Name: "A stream overview works",
			DB:   true,
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config:      config.EnvironmentConfig{},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"development": "v1",
					},
					Version: 1,
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[types.EnvName]string{
						"development": "v2",
					},
					Version: 2,
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
			},
			Test: func(t *testing.T, svc *OverviewServiceServer) {
				ctx, cancel := context.WithCancel(testutil.MakeTestContext())
				ch := make(chan *api.GetOverviewResponse)
				stream := mockOverviewService_StreamOverviewServer{
					Results: ch,
					Ctx:     ctx,
				}
				wg := sync.WaitGroup{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := svc.StreamOverview(&api.GetOverviewRequest{}, &stream)
					if err != nil {
						t.Error(err)
					}
				}()

				// Check that we get a first overview
				overview1 := <-ch
				if overview1 == nil {
					t.Fatal("overview is nil")
				}

				// Update a version and see that the version changed
				err := svc.Repository.Apply(ctx, &repository.CreateEnvironmentLock{
					Environment:           "development",
					LockId:                "ov-test",
					Message:               "stream overview test",
					CiLink:                "",
					AllowedDomains:        []string{},
					TransformerEslVersion: 0,
				})

				if err != nil {
					t.Fatal(err)
				}

				// Check that the second overview is different
				overview2 := <-ch
				if overview2 == nil {
					t.Fatal("overview is nil")
				}

				cancel()
				wg.Wait()
			},
		},
		{
			Name: "Test with DB",
			DB:   true,
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: dev,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
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
					WriteCommitData:       false,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           "test",
					Manifests: map[types.EnvName]string{
						dev: "v1",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       false,
					PreviousCommit:        "",
					TransformerEslVersion: 2,
					Application:           "test",
					Manifests: map[types.EnvName]string{
						dev: "v2",
					},
				},
			},
			Test: func(t *testing.T, svc *OverviewServiceServer) {
				var ctx = auth.WriteUserToContext(testutil.MakeTestContext(), auth.User{
					Email: "test-email@example.com",
					Name:  "overview tester",
				})
				resp, err := svc.GetOverview(ctx, &api.GetOverviewRequest{})
				if err != nil {
					t.Fatal(err)
				}
				if resp.GitRevision == "" {
					t.Errorf("expected non-empty git revision but was empty")
				}

				const expectedEnvs = 1
				if len(resp.EnvironmentGroups) != expectedEnvs {
					t.Errorf("expected %d environmentGroups, got %q", expectedEnvs, resp.EnvironmentGroups)
				}
				devGroup := resp.EnvironmentGroups[0]
				if devGroup.EnvironmentGroupName != "dev" {
					t.Errorf("dev environmentGroup has wrong name: %q", devGroup.EnvironmentGroupName)
				}
				dev := devGroup.Environments[0]
				if dev.Name != "dev" {
					t.Errorf("development environment has wrong name: %q", dev.Name)
				}
				if dev.Config.Upstream == nil {
					t.Errorf("development environment has wrong upstream: %#v", dev.Config.Upstream)
				} else {
					if !dev.Config.Upstream.GetLatest() {
						t.Errorf("production environment has wrong upstream: %#v", dev.Config.Upstream)
					}
				}

				// Check applications
				app, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test"})
				if err != nil {
					t.Errorf("got an error fetching app deails")
				}

				test := app.Application

				if test.Name != "test" {
					t.Errorf("test applications name is not test but %q", test.Name)
				}
				if len(test.Releases) != 2 {
					t.Errorf("expected two releases, got %v", test.Releases)
				}
				actualRelease := test.Releases[1]
				if actualRelease.Version != 1 {
					t.Errorf("expected test release version to be 1, but got %d", test.Releases[1].Version)
				}
				if actualRelease.SourceAuthor != "example <example@example.com>" {
					t.Errorf("expected test source author to be \"example <example@example.com>\", but got %q", actualRelease.SourceAuthor)
				}
				if actualRelease.SourceMessage != "changed something (#678)" {
					t.Errorf("expected test source message to be \"changed something\", but got %q", actualRelease.SourceMessage)
				}
				if actualRelease.SourceCommitId != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
					t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", actualRelease.SourceCommitId)
				}

				//Check cache
				if _, err := svc.GetOverview(ctx, &api.GetOverviewRequest{}); err != nil {
					t.Errorf("expected no error getting overview from cache, got %q", err)
				}
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			if tc.DB {
				var err error
				repo, err = setupRepositoryTestWithDB(t)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var err error
				repo, err = setupRepositoryTest(t)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			ctx := testutil.MakeTestContext()
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    ctx,
			}
			tc.Test(t, svc)
			close(shutdown)
		})
	}
}

func TestGetApplicationDetails(t *testing.T) {
	//var dev = "dev"
	var env types.EnvName = "development"
	var secondEnv types.EnvName = "development2"
	var stagingGroup types.EnvName = "stagingGroup"
	var thirdEnv types.EnvName = "staging"
	var appName = "test-app"
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		AppName          string
		ExpectedResponse *api.GetAppDetailsResponse
	}{
		{
			Name:    "Get App details",
			AppName: appName,
			ExpectedResponse: &api.GetAppDetailsResponse{
				Application: &api.Application{
					Name: appName,
					Releases: []*api.Release{
						{
							Version:        1,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
					},
					Team: "team-123",
				},
				Deployments: map[string]*api.Deployment{
					string(env): {
						Version:         1,
						QueuedVersion:   0,
						UndeployVersion: false,
						DeploymentMetaData: &api.Deployment_DeploymentMetaData{
							DeployAuthor: "test tester",
						},
					},
				},
				TeamLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-team-lock",
								Message: "team lock for team 123",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
				AppLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-app-lock",
								Message: "app lock for test-app",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(env),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(secondEnv),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateEnvironmentTeamLock{
					Team:        "team-123",
					Environment: env,
					LockId:      "my-team-lock",
					Message:     "team lock for team 123",
				},

				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
			},
		},
		{
			Name:    "Get App details returns deleted apps",
			AppName: appName,
			ExpectedResponse: &api.GetAppDetailsResponse{
				Application: &api.Application{
					Name:     appName,
					Releases: []*api.Release{},
					Team:     "team-123",
				},
				Deployments: map[string]*api.Deployment{},
				TeamLocks:   map[string]*api.Locks{},
				AppLocks:    map[string]*api.Locks{},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(env),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(secondEnv),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateUndeployApplicationVersion{
					Authentication:        repository.Authentication{},
					Application:           appName,
					WriteCommitData:       true,
					TransformerEslVersion: 1,
				},
				&repository.UndeployApplication{
					Authentication:        repository.Authentication{},
					Application:           appName,
					TransformerEslVersion: 1,
				},
			},
		},
		{
			Name:    "Get App details doesn't return deployments on releases without the corresponding environment",
			AppName: appName,
			ExpectedResponse: &api.GetAppDetailsResponse{
				Application: &api.Application{
					Name: appName,
					Releases: []*api.Release{
						{
							Version:        3,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(thirdEnv)},
						},
						{
							Version:        2,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							IsMinor:        true,
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env)},
						},
						{
							Version:        1,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env)},
						},
					},
					Team: "team-123",
				},
				TeamLocks: map[string]*api.Locks{},
				AppLocks:  map[string]*api.Locks{},
				Deployments: map[string]*api.Deployment{
					string(env): {
						Version:         3,
						QueuedVersion:   0,
						UndeployVersion: false,
						DeploymentMetaData: &api.Deployment_DeploymentMetaData{
							DeployAuthor: "test tester",
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(env),
					},
				},
				&repository.CreateEnvironment{
					Environment: thirdEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: env,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(stagingGroup),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:      "v1",
						thirdEnv: "v2",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:      "v1",
						thirdEnv: "v2",
					},
				},
				&repository.DeployApplicationVersion{
					Environment: thirdEnv,
					Application: appName,
					Version:     1,
				},
				&repository.DeleteEnvFromApp{
					Application: appName,
					Environment: thirdEnv,
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               3,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:      "v1",
						thirdEnv: "v2",
					},
				},
			},
		},
		{
			Name:    "Get App details - Revisions",
			AppName: appName,
			ExpectedResponse: &api.GetAppDetailsResponse{
				Application: &api.Application{
					Name: appName,
					Releases: []*api.Release{
						{
							Version:        2,
							Revision:       0,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
						{
							Version:        1,
							Revision:       2,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
						{
							Version:        1,
							Revision:       1,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
					},
					Team: "team-123",
				},
				Deployments: map[string]*api.Deployment{
					string(env): {
						Version:         2,
						QueuedVersion:   0,
						UndeployVersion: false,
						DeploymentMetaData: &api.Deployment_DeploymentMetaData{
							DeployAuthor: "test tester",
						},
					},
				},
				TeamLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-team-lock",
								Message: "team lock for team 123",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
				AppLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-app-lock",
								Message: "app lock for test-app",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(env),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(secondEnv),
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					Revision:              1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					Revision:              0,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 2,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v3",
						secondEnv: "v4",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               1,
					Revision:              2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 3,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v5",
						secondEnv: "v6",
					},
				},
				&repository.CreateEnvironmentTeamLock{
					Team:        "team-123",
					Environment: env,
					LockId:      "my-team-lock",
					Message:     "team lock for team 123",
				},

				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
			},
		},
		{
			Name:    "Get App details - Revisions",
			AppName: appName,
			ExpectedResponse: &api.GetAppDetailsResponse{
				Application: &api.Application{
					Name: appName,
					Releases: []*api.Release{
						{
							Version:        2,
							Revision:       3,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
						{
							Version:        2,
							Revision:       2,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
						{
							Version:        2,
							Revision:       1,
							SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
							SourceAuthor:   "example <example@example.com>",
							SourceMessage:  "changed something (#678)",
							PrNumber:       "678",
							CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
							Environments:   []string{string(env), string(secondEnv)},
						},
					},
					Team: "team-123",
				},
				Deployments: map[string]*api.Deployment{
					string(env): {
						Version:         2,
						QueuedVersion:   0,
						UndeployVersion: false,
						DeploymentMetaData: &api.Deployment_DeploymentMetaData{
							DeployAuthor: "test tester",
						},
						Revision: 3,
					},
				},
				TeamLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-team-lock",
								Message: "team lock for team 123",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
				AppLocks: map[string]*api.Locks{
					"development": {
						Locks: []*api.Lock{
							{
								LockId:  "my-app-lock",
								Message: "app lock for test-app",
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(env),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(secondEnv),
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					Revision:              2,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					Revision:              3,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 2,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v3",
						secondEnv: "v4",
					},
				},
				&repository.CreateApplicationVersion{
					Authentication:        repository.Authentication{},
					Version:               2,
					Revision:              1,
					SourceCommitId:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceAuthor:          "example <example@example.com>",
					SourceMessage:         "changed something (#678)",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       true,
					PreviousCommit:        "",
					TransformerEslVersion: 3,
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v5",
						secondEnv: "v6",
					},
				},
				&repository.CreateEnvironmentTeamLock{
					Team:        "team-123",
					Environment: env,
					LockId:      "my-team-lock",
					Message:     "team lock for team 123",
				},

				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}
			config := repository.RepositoryConfig{
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			svc := &OverviewServiceServer{
				Repository:       repo,
				RepositoryConfig: config,
				DBHandler:        repo.State().DBHandler,
				Shutdown:         shutdown,
			}

			if err := repo.Apply(testutil.MakeTestContext(), tc.Setup...); err != nil {
				t.Fatal(err)
			}

			var ctx = auth.WriteUserToContext(testutil.MakeTestContext(), auth.User{
				Email: "app-email@example.com",
				Name:  "overview tester",
			})

			resp, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: appName})
			if err != nil {
				t.Fatal(err)
			}

			app := resp.Application
			expected := tc.ExpectedResponse
			if diff := cmp.Diff(app.Name, expected.Application.Name); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			//Releases
			if diff := cmp.Diff(expected.Application.Releases, resp.Application.Releases, cmpopts.IgnoreUnexported(api.Release{}), cmpopts.IgnoreFields(api.Release{}, "CreatedAt")); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			//Deployments
			environmentsToCheck := []string{string(env), string(thirdEnv)}
			for _, environmentToCheck := range environmentsToCheck {
				t.Logf("Checking %s", environmentToCheck)
				expectedDeployment := expected.Deployments[environmentToCheck]
				resultDeployment := resp.Deployments[environmentToCheck]
				if diff := cmp.Diff(expectedDeployment, resultDeployment, cmpopts.IgnoreUnexported(api.Deployment{}), cmpopts.IgnoreUnexported(api.Deployment_DeploymentMetaData{}), cmpopts.IgnoreFields(api.Deployment_DeploymentMetaData{}, "DeployTime")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}

			//Locks
			if diff := cmp.Diff(expected.AppLocks, resp.AppLocks, cmpopts.IgnoreUnexported(api.Locks{}), cmpopts.IgnoreUnexported(api.Lock{}), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt"), cmpopts.IgnoreUnexported(api.Actor{})); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(expected.TeamLocks, resp.TeamLocks, cmpopts.IgnoreUnexported(api.Locks{}), cmpopts.IgnoreUnexported(api.Lock{}), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt"), cmpopts.IgnoreUnexported(api.Actor{})); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			close(shutdown)
		})
	}
}

func TestGetAllAppLocks(t *testing.T) {
	var dev types.EnvName = "dev"
	var env types.EnvName = "development"
	var secondEnv types.EnvName = "development2"
	var appName = "test-app"
	var anotherAppName = "another-app-name"
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		AppName          string
		ExpectedResponse *api.GetAllAppLocksResponse
	}{
		{
			Name:    "Get All Locks",
			AppName: appName,
			ExpectedResponse: &api.GetAllAppLocksResponse{
				AllAppLocks: map[string]*api.AllAppLocks{
					string(env): {
						AppLocks: map[string]*api.Locks{
							appName: {
								Locks: []*api.Lock{
									{
										LockId:  "my-app-lock",
										Message: "app lock for test-app",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
						},
					},
					string(secondEnv): {
						AppLocks: map[string]*api.Locks{
							appName: {
								Locks: []*api.Lock{
									{
										LockId:  "my-app-lock",
										Message: "app lock for test-app",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: secondEnv,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
			},
		},
		{
			Name:    "Get All Locks - no locks",
			AppName: appName,
			ExpectedResponse: &api.GetAllAppLocksResponse{
				AllAppLocks: map[string]*api.AllAppLocks{},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
			},
		},
		{
			Name:    "Get All Locks - multiple locks per environment",
			AppName: appName,
			ExpectedResponse: &api.GetAllAppLocksResponse{
				AllAppLocks: map[string]*api.AllAppLocks{
					string(secondEnv): {
						AppLocks: map[string]*api.Locks{
							appName: {
								Locks: []*api.Lock{
									{
										LockId:  "my-app-lock",
										Message: "app lock for test-app",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
						},
					},
					string(env): {
						AppLocks: map[string]*api.Locks{
							appName: {
								Locks: []*api.Lock{
									{
										LockId:  "A-my-app-lock",
										Message: "app lock for test-app (1) on env",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
									{
										LockId:  "B-duplicate-app-lock",
										Message: "app lock for test-app (2) on env",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
							anotherAppName: {
								Locks: []*api.Lock{
									{
										LockId:  "my-app-lock",
										Message: "my-app-lock message on anotherAppName",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
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
					Application:           appName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
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
					Application:           anotherAppName,
					Manifests: map[types.EnvName]string{
						env:       "v1",
						secondEnv: "v2",
					},
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "A-my-app-lock",
					Message:     "app lock for test-app (1) on env",
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: env,
					LockId:      "B-duplicate-app-lock",
					Message:     "app lock for test-app (2) on env",
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: anotherAppName,
					Environment: env,
					LockId:      "my-app-lock",
					Message:     "my-app-lock message on anotherAppName",
				},
				&repository.CreateEnvironmentApplicationLock{
					Application: appName,
					Environment: secondEnv,
					LockId:      "my-app-lock",
					Message:     "app lock for test-app",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository

			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}
			config := repository.RepositoryConfig{
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			svc := &OverviewServiceServer{
				Repository:       repo,
				RepositoryConfig: config,
				DBHandler:        repo.State().DBHandler,
				Shutdown:         shutdown,
			}

			if err := repo.Apply(testutil.MakeTestContext(), tc.Setup...); err != nil {
				t.Fatal(err)
			}

			var ctx = auth.WriteUserToContext(testutil.MakeTestContext(), auth.User{
				Email: "app-email@example.com",
				Name:  "overview tester",
			})

			resp, err := svc.GetAllAppLocks(ctx, &api.GetAllAppLocksRequest{})
			if err != nil {
				t.Fatal(err)
			}

			//Locks
			if diff := cmp.Diff(tc.ExpectedResponse, resp, cmpopts.IgnoreUnexported(api.GetAllAppLocksResponse{}), cmpopts.IgnoreUnexported(api.Locks{}), cmpopts.IgnoreUnexported(api.Lock{}), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt"), cmpopts.IgnoreUnexported(api.Actor{}), cmpopts.IgnoreUnexported(api.AllAppLocks{})); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			close(shutdown)
		})
	}
}
func TestGetAllEnvTeamLocks(t *testing.T) {
	var dev types.EnvName = "dev"
	var env types.EnvName = "development"
	var secondEnv types.EnvName = "development2"
	var team = "team"
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		ExpectedResponse *api.GetAllEnvTeamLocksResponse
	}{
		{
			Name: "Get All Locks",
			ExpectedResponse: &api.GetAllEnvTeamLocksResponse{
				AllEnvLocks: map[string]*api.Locks{
					string(env): &api.Locks{
						Locks: []*api.Lock{
							{
								Message:   "message1",
								LockId:    "lockId1",
								CreatedAt: &timestamppb.Timestamp{},
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
					string(secondEnv): &api.Locks{
						Locks: []*api.Lock{
							{
								Message:   "message2",
								LockId:    "lockId2",
								CreatedAt: &timestamppb.Timestamp{},
								CreatedBy: &api.Actor{
									Name:  "test tester",
									Email: "testmail@example.com",
								},
							},
						},
					},
				},
				AllTeamLocks: map[string]*api.AllTeamLocks{
					string(env): &api.AllTeamLocks{
						TeamLocks: map[string]*api.Locks{
							team: &api.Locks{
								Locks: []*api.Lock{
									{
										Message:   "message3",
										LockId:    "lockId3",
										CreatedAt: &timestamppb.Timestamp{},
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
							},
						},
					},
				},
			},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: env,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: secondEnv,
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironmentLock{
					Environment: env,
					LockId:      "lockId1",
					Message:     "message1",
				},
				&repository.CreateEnvironmentLock{
					Environment: secondEnv,
					LockId:      "lockId2",
					Message:     "message2",
				},
				&repository.CreateEnvironmentTeamLock{
					Environment: env,
					LockId:      "lockId3",
					Message:     "message3",
					Team:        team,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}
			config := repository.RepositoryConfig{
				ArgoCdGenerateFiles: true,
				DBHandler:           repo.State().DBHandler,
			}
			svc := &OverviewServiceServer{
				Repository:       repo,
				RepositoryConfig: config,
				DBHandler:        repo.State().DBHandler,
				Shutdown:         shutdown,
			}

			if err := repo.Apply(testutil.MakeTestContext(), tc.Setup...); err != nil {
				t.Fatal(err)
			}

			var ctx = auth.WriteUserToContext(testutil.MakeTestContext(), auth.User{
				Email: "app-email@example.com",
				Name:  "overview tester",
			})

			resp, err := svc.GetAllEnvTeamLocks(ctx, &api.GetAllEnvTeamLocksRequest{})
			if err != nil {
				t.Fatal(err)
			}

			//Locks
			if diff := cmp.Diff(tc.ExpectedResponse, resp, cmpopts.IgnoreUnexported(api.GetAllEnvTeamLocksResponse{}), cmpopts.IgnoreUnexported(api.Locks{}), cmpopts.IgnoreUnexported(api.Lock{}), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt"), cmpopts.IgnoreUnexported(api.Actor{}), cmpopts.IgnoreUnexported(api.AllTeamLocks{})); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			close(shutdown)
		})
	}
}

func TestDeriveUndeploySummary(t *testing.T) {
	var tcs = []struct {
		Name           string
		AppName        string
		Deployments    map[string]*api.Deployment
		ExpectedResult api.UndeploySummary
	}{
		{
			Name:           "No Environments",
			AppName:        "foo",
			Deployments:    map[string]*api.Deployment{},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "one Environment but no Application",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"bar": { // different app
					UndeployVersion: true,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with undeploy",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"foo": {
					UndeployVersion: true,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with normal version",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"foo": {
					UndeployVersion: false,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all undeploy",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"foo": {
					UndeployVersion: true,
					Version:         666,
				},
				"bar": {
					UndeployVersion: true,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "Two Envs all normal",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"foo": {
					UndeployVersion: false,
					Version:         666,
				},
				"bar": {
					UndeployVersion: false,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all different",
			AppName: "foo",
			Deployments: map[string]*api.Deployment{
				"foo": {
					UndeployVersion: false,
					Version:         666,
				},
				"bar": {
					UndeployVersion: true,
					Version:         666,
				},
			},
			ExpectedResult: api.UndeploySummary_MIXED,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			actualResult := deriveUndeploySummary(tc.AppName, tc.Deployments)
			if !cmp.Equal(tc.ExpectedResult, actualResult) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedResult, actualResult))
			}
		})
	}
}

func TestDeploymentAttemptsGetAppDetails(t *testing.T) {
	var dev = "dev"
	tcs := []struct {
		Name                   string
		Setup                  []repository.Transformer
		AppsNamesToCheck       []string
		ExpectedCachedOverview *api.GetOverviewResponse
		ExpectedAppDetails     map[string]*api.GetAppDetailsResponse //appName -> appDetails
	}{
		{
			Name: "Update App Details Deployment Attempt",
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
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "development",
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateEnvironmentApplicationLock{
					Environment: "development",
					Application: "test",
					LockId:      "manual",
					Message:     "no",
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     2,
					Manifests: map[types.EnvName]string{
						"development": "dev-2",
					},
					Team:           "test-team",
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
			},
			ExpectedAppDetails: map[string]*api.GetAppDetailsResponse{
				"test": {
					Application: &api.Application{
						Name: "test",
						Releases: []*api.Release{
							{
								Version:        2,
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
								Environments:   []string{"development"},
							},
							{
								Version:        1,
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
								Environments:   []string{"development"},
							},
						},
						Team:     "test-team",
						Warnings: []*api.Warning{},
					},
					Deployments: map[string]*api.Deployment{
						"development": {
							Version:       1,
							QueuedVersion: 2,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{
								DeployAuthor: "test tester",
								CiLink:       "",
							},
						},
					},
					AppLocks: map[string]*api.Locks{
						"development": {
							Locks: []*api.Lock{
								{
									Message: "no",
									LockId:  "manual",
									CreatedBy: &api.Actor{
										Name:  "test tester",
										Email: "testmail@example.com",
									},
								},
							},
						},
					},
					TeamLocks: map[string]*api.Locks{},
				},
			},
			AppsNamesToCheck: []string{"test"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutil.MakeTestContext()
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}

			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    ctx,
			}
			for _, currentAppName := range tc.AppsNamesToCheck {
				response, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: currentAppName})

				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(tc.ExpectedAppDetails[currentAppName], response, getAppDetailsIgnoredTypes(), cmpopts.IgnoreFields(api.Release{}, "CreatedAt"), cmpopts.IgnoreFields(api.Deployment_DeploymentMetaData{}, "DeployTime"), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt")); diff != "" {
					t.Errorf("response missmatch (-want, +got): %s", diff)
				}
			}
		})

	}
}

func TestCalculateWarnings(t *testing.T) {
	var dev types.EnvName = "dev"
	tcs := []struct {
		Name             string
		ExpectedWarnings map[string][]*api.Warning //appName -> expected warnings
		Setup            []repository.Transformer
		AppsNamesToCheck []string
	}{
		{
			Name:             "no deployments - no warning",
			AppsNamesToCheck: []string{"test"},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{ //Not upstream latest!
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
			},
			ExpectedWarnings: map[string][]*api.Warning{
				"test": {},
			},
		},
		{
			Name:             "app deployed in higher version on upstream should warn",
			AppsNamesToCheck: []string{"test"},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "development",
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"development": "dev",
						"staging":     "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     2,
					Manifests: map[types.EnvName]string{
						"development": "dev-2",
						"staging":     "staging-2",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     1,
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "staging",
					Version:     2,
				},
			},
			ExpectedWarnings: map[string][]*api.Warning{
				"test": {
					{
						WarningType: &api.Warning_UnusualDeploymentOrder{
							UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
								UpstreamVersion:     1,
								UpstreamEnvironment: "development",
								ThisVersion:         2,
								ThisEnvironment:     "staging",
							},
						},
					},
				},
			},
		},
		{
			Name:             "app deployed in same version on upstream should not warn",
			AppsNamesToCheck: []string{"test"},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "development",
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"development": "dev",
						"staging":     "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     2,
					Manifests: map[types.EnvName]string{
						"development": "dev-2",
						"staging":     "staging-2",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "staging",
					Version:     2,
				},
			},
			ExpectedWarnings: map[string][]*api.Warning{
				"test": {},
			},
		},
		{
			Name:             "app deployed in no version on upstream should warn",
			AppsNamesToCheck: []string{"test"},
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: types.StringPtr(dev),
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "development",
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[types.EnvName]string{
						"development": "dev",
						"staging":     "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
					Environment: "staging",
					Version:     1,
				},
			},
			ExpectedWarnings: map[string][]*api.Warning{
				"test": {
					{
						WarningType: &api.Warning_UpstreamNotDeployed{
							UpstreamNotDeployed: &api.UpstreamNotDeployed{
								UpstreamEnvironment: "development",
								ThisVersion:         1,
								ThisEnvironment:     "staging",
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}

			for _, tr := range tc.Setup {
				if err := repo.Apply(testutil.MakeTestContext(), tr); err != nil {
					t.Fatal(err)
				}
			}
			ctx := testutil.MakeTestContext()
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    ctx,
			}

			for _, appName := range tc.AppsNamesToCheck {
				appDetails, err := svc.GetAppDetails(testutil.MakeTestContext(), &api.GetAppDetailsRequest{AppName: appName})
				if err != nil {
					t.Error(err)
				}

				if diff := cmp.Diff(tc.ExpectedWarnings[appName], appDetails.Application.Warnings, getAppDetailsIgnoredTypes()); diff != "" {
					t.Errorf("response missmatch (-want, +got): %s", diff)
				}
			}
		})
	}
}

func TestDeploymentHistory(t *testing.T) {
	const testApp = "test-app"
	const testApp2 = "test-app-2"
	created := time.Now()
	today := created.Round(time.Hour * 24)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := yesterday.AddDate(0, 0, 2)
	versionOne := uint64(1)
	versionTwo := uint64(2)
	dev := "dev"
	staging := "staging"

	tcs := []struct {
		Name             string
		Setup            []db.Deployment
		SetupReleases    []db.DBReleaseWithMetaData
		SetupEnvs        []repository.Transformer
		Request          *api.DeploymentHistoryRequest
		ExpectedCsvLines []string
		ExpectedError    string
	}{
		{
			Name:  "Test empty deployment history",
			Setup: []db.Deployment{},
			SetupEnvs: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
			},
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday),
				EndDate:     timestamppb.New(tomorrow),
				Environment: "dev",
			},
			ExpectedCsvLines: []string{
				"time,app,environment,deployed release version,source repository commit hash,previous release version\n",
			},
		},
		{
			Name: "Test non-empty deployment history",
			Setup: []db.Deployment{
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "staging",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp2,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
			},
			SetupEnvs: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     1,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp2,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "cccccccccccccccccccccccccccccccccccccccc",
					SourceMessage:  "changed something (#678)",
				},
			},
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday),
				EndDate:     timestamppb.New(tomorrow),
				Environment: "dev",
			},
			ExpectedCsvLines: []string{
				"time,app,environment,deployed release version,source repository commit hash,previous release version\n",
				"test-app,dev,1,aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,nil\n",
				"test-app-2,dev,2,cccccccccccccccccccccccccccccccccccccccc,nil\n",
				"test-app,dev,2,bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb,1\n",
			},
		},
		{
			Name: "Test non-empty deployment history for a different environment",
			Setup: []db.Deployment{
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "staging",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp2,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
			},
			SetupEnvs: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     1,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp2,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "cccccccccccccccccccccccccccccccccccccccc",
					SourceMessage:  "changed something (#678)",
				},
			},
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday),
				EndDate:     timestamppb.New(tomorrow),
				Environment: "staging",
			},
			ExpectedCsvLines: []string{
				"time,app,environment,deployed release version,source repository commit hash,previous release version\n",
				"test-app,staging,1,aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,nil\n",
			},
		},
		{
			Name: "we are still able to retrieve commit hash from a very old release",
			Setup: []db.Deployment{
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "staging",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp2,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
				{
					Created: created,
					Env:     "dev",
					App:     testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionTwo,
					},
					TransformerID: 0,
				},
			},
			SetupReleases: []db.DBReleaseWithMetaData{
				{
					App: testApp,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					Manifests: db.DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev":     "dev",
							"staging": "staging",
						},
					},
					Environments: []types.EnvName{"dev", "staging"},
					Metadata: db.DBReleaseMetaData{
						SourceAuthor:   "example <example@example.com>",
						SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						SourceMessage:  "changed something (#678)",
					},
					Created: created.AddDate(-1, 0, 0),
				},
			},
			SetupEnvs: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     1,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp2,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "cccccccccccccccccccccccccccccccccccccccc",
					SourceMessage:  "changed something (#678)",
				},
			},
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday),
				EndDate:     timestamppb.New(tomorrow),
				Environment: "staging",
			},
			ExpectedCsvLines: []string{
				"time,app,environment,deployed release version,source repository commit hash,previous release version\n",
				"test-app,staging,1,aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa,nil\n",
			},
		},
		{
			Name: "Test no deployment in the specified time frame",
			Setup: []db.Deployment{
				{
					Created: created,
					Env:     "dev",
					App:     "testapp",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  &versionOne,
					},
					TransformerID: 0,
				},
			},
			SetupEnvs: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "dev",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						ArgoCd:           nil,
						EnvironmentGroup: &staging,
					},
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     1,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: testApp2,
					Version:     2,
					Manifests: map[types.EnvName]string{
						"dev":     "dev",
						"staging": "staging",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "cccccccccccccccccccccccccccccccccccccccc",
					SourceMessage:  "changed something (#678)",
				},
			},
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday.AddDate(0, 0, -4)),
				EndDate:     timestamppb.New(tomorrow.AddDate(0, 0, -4)),
				Environment: "dev",
			},
			ExpectedCsvLines: []string{
				"time,app,environment,deployed release version,previous release version\n",
			},
		},
		{
			Name:          "Test end date before start date error",
			Setup:         []db.Deployment{},
			ExpectedError: fmt.Sprintf("end date (%s) happens before start date (%s)", yesterday.Format(time.DateOnly), tomorrow.Format(time.DateOnly)),
			Request: &api.DeploymentHistoryRequest{
				StartDate: timestamppb.New(tomorrow),
				EndDate:   timestamppb.New(yesterday),
			},
		},
		{
			Name:          "Test time frame from today to yesterday",
			Setup:         []db.Deployment{},
			ExpectedError: fmt.Sprintf("end date (%s) happens before start date (%s)", yesterday.Format(time.DateOnly), today.Format(time.DateOnly)),
			Request: &api.DeploymentHistoryRequest{
				StartDate: timestamppb.New(today),
				EndDate:   timestamppb.New(yesterday),
			},
		},
		{
			Name:          "Test with environment that does not exist",
			Setup:         []db.Deployment{},
			SetupEnvs:     []repository.Transformer{},
			ExpectedError: `environment "dev" does not exist`,
			Request: &api.DeploymentHistoryRequest{
				StartDate:   timestamppb.New(yesterday),
				EndDate:     timestamppb.New(tomorrow),
				Environment: "dev",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			var repo repository.Repository
			repo, err := setupRepositoryTestWithDB(t)
			if err != nil {
				t.Fatal(err)
			}

			ctx := testutil.MakeTestContext()
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    ctx,
			}

			for _, tr := range tc.SetupEnvs {
				if err := repo.Apply(ctx, tr); err != nil {
					t.Fatal(err)
				}
			}

			err = svc.DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := svc.DBHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				for _, release := range tc.SetupReleases {
					if err := svc.DBHandler.DBUpdateOrCreateRelease(ctx, transaction, release); err != nil {
						return err
					}
				}

				for _, deployment := range tc.Setup {
					if err := svc.DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, deployment); err != nil {
						return err
					}
				}

				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			var expectedLinesWithCreated []string
			expectedLinesWithCreated = append(expectedLinesWithCreated, "time,app,environment,deployed release version,source repository commit hash,previous release version\n")

			err = svc.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				query := svc.DBHandler.AdaptQuery(`
					SELECT created FROM deployments_history
					WHERE releaseversion IS NOT NULL
					ORDER BY created ASC;
				`)

				rows, err := transaction.QueryContext(ctx, query)
				if err != nil {
					return err
				}

				defer func() { _ = rows.Close() }()
				for i := 1; rows.Next() && i < len(tc.ExpectedCsvLines); i++ {
					var createdAt time.Time
					err = rows.Scan(&createdAt)
					if err != nil {
						return fmt.Errorf("error scanning row: %w", err)
					}
					line := fmt.Sprintf("%s,%s", createdAt.Format(time.RFC3339), tc.ExpectedCsvLines[i])
					expectedLinesWithCreated = append(expectedLinesWithCreated, line)
				}

				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			ch := make(chan *api.DeploymentHistoryResponse)
			stream := mockOverviewService_DeploymentHistoryServer{
				Results: ch,
				Ctx:     ctx,
			}

			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := svc.StreamDeploymentHistory(tc.Request, &stream)
				close(ch)
				if err != nil {
					if diff := cmp.Diff(tc.ExpectedError, err.Error()); diff != "" {
						t.Errorf("error mismatch (-want, +got):\n%s", diff)
					}
				}
			}()

			var got []string
			expectedLineCount := len(tc.ExpectedCsvLines)
			if expectedLineCount == 0 {
				expectedLineCount = 1
			}

			line := 1
			for res := range ch {
				expectedProgress := uint32(line * 100 / expectedLineCount)
				if res.Progress != expectedProgress {
					t.Errorf("deployment history progress mismatch: expected %d and got %d", expectedProgress, res.Progress)
				}

				line++
				got = append(got, res.Deployment)
			}

			if diff := cmp.Diff(expectedLinesWithCreated, got); tc.ExpectedError == "" && diff != "" {
				t.Errorf("deployment history csv lines mismatch (-want, +got):\n%s", diff)
			}

			wg.Wait()
			close(shutdown)
		})
	}
}

func getAppDetailsIgnoredTypes() cmp.Option {
	return cmpopts.IgnoreUnexported(api.GetAppDetailsResponse{},
		api.Deployment{},
		api.Application{},
		api.Locks{},
		api.Lock{},
		api.Warning{},
		api.Warning_UnusualDeploymentOrder{},
		api.UnusualDeploymentOrder{},
		api.UpstreamNotDeployed{},
		timestamppb.Timestamp{},
		api.Warning_UpstreamNotDeployed{},
		api.Release{},
		api.Lock{},
		api.Actor{},
		api.Deployment_DeploymentMetaData{})
}
func getOverviewIgnoredTypes() cmp.Option {
	return cmpopts.IgnoreUnexported(api.GetOverviewResponse{},
		api.EnvironmentGroup{},
		api.Environment{},
		api.Application{},
		api.Warning{},
		api.Warning_UnusualDeploymentOrder{},
		api.UnusualDeploymentOrder{},
		api.UpstreamNotDeployed{},
		api.Warning_UpstreamNotDeployed{},
		api.Release{},
		api.EnvironmentConfig_ArgoCD_Destination{},
		timestamppb.Timestamp{},
		api.EnvironmentConfig{},
		api.EnvironmentConfig_Upstream{},
		api.EnvironmentConfig_ArgoCD{},
		api.Lock{},
		api.Actor{},
		api.OverviewApplication{},
		api.Deployment{},
		api.Locks{})
}
