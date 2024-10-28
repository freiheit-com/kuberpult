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
	"sync"
	"testing"
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

func TestOverviewService(t *testing.T) {
	var dev = "dev"
	var development = "development"
	var staging = "staging"
	var prod = "production"
	var upstreamLatest = true
	tcs := []struct {
		Name                   string
		Setup                  []repository.Transformer
		Test                   func(t *testing.T, svc *OverviewServiceServer)
		DB                     bool
		ExpectedCachedOverview *api.GetOverviewResponse
		ExpectedAppDetails     map[string]*api.GetAppDetailsResponse //appName -> appDetails
	}{
		{
			Name: "A simple overview works",
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
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-team",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
					Team: "test-team",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-incorrect-pr-number",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					SourceMessage:  "changed something (#678",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-only-pr-number",
					Version:     1,
					Manifests: map[string]string{
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
					Version:     1,
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
							},
						},
					},
					Deployments: map[string]*api.Deployment{
						"prod": {
							Version:            1,
							QueuedVersion:      0,
							UndeployVersion:    false,
							DeploymentMetaData: &api.Deployment_DeploymentMetaData{},
						},
					},
				},
				"test-with-team": {
					Application: &api.Application{
						Name: "test",
						Team: "test-team",
					},
					Deployments: map[string]*api.Deployment{
						"dev": {},
					},
				},
			},
			DB: true,
			ExpectedCachedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: development,
								Locks: map[string]*api.Lock{
									"manual": {
										Message: "please",
										LockId:  "manual",
										CreatedBy: &api.Actor{
											Name:  "test tester",
											Email: "testmail@example.com",
										},
									},
								},
								TeamLocks: map[string]*api.Locks{
									"test-team": {
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
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{},
									},
									EnvironmentGroup: &dev,
								},
								Priority: api.Priority_UPSTREAM,
							},
						},
						Priority: api.Priority_UPSTREAM,
					},
					{
						EnvironmentGroupName: staging,
						Environments: []*api.Environment{
							{
								Name: staging,
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: &development,
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{},
									},
									EnvironmentGroup: &staging,
								},
								DistanceToUpstream: 1,
								Priority:           api.Priority_PRE_PROD,
							},
						},
						Priority:           api.Priority_PRE_PROD,
						DistanceToUpstream: 1,
					},
					{
						EnvironmentGroupName: prod,
						Environments: []*api.Environment{
							{
								Name: prod,
								AppLocks: map[string]*api.Locks{
									"test": {
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
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Environment: &staging,
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{},
									},
									EnvironmentGroup: &prod,
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
						Name: "test-with-team",
						Team: "test-team",
					},
					{
						Name: "test-with-incorrect-pr-number",
						Team: "",
					},
					{
						Name: "test-with-only-pr-number",
						Team: "",
					},
				},
				GitRevision: "0",
			},
			Test: func(t *testing.T, svc *OverviewServiceServer) {
				//TODO: This test suite has some commented out sections. These tests should either be adapted or reimplemented in Ref: SRX-9PBRYS.
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

				const expectedEnvs = 3
				if len(resp.EnvironmentGroups) != expectedEnvs {
					t.Errorf("expected %d environmentGroups, got %q", expectedEnvs, resp.EnvironmentGroups)
				}
				app, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test"})
				if err != nil {
					t.Errorf("Error fetching information for app test: %v", err)
				}
				testApp := app.Application

				releases := app.Application.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "678" {
					t.Errorf("Release should have PR number \"678\", but got %q", releases[0].PrNumber)
				}

				app, err = svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test-with-team"})
				if err != nil {
					t.Errorf("Error fetching information for app test")
				}
				testApp = app.Application
				if testApp.SourceRepoUrl != "" {
					t.Errorf("Expected \"\", but got %#q", testApp.SourceRepoUrl)
				}
				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "" {
					t.Errorf("Release should not have PR number")
				}

				app, err = svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test-with-incorrect-pr-number"})
				if err != nil {
					t.Errorf("Error fetching information for app test")
				}
				testApp = app.Application

				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "" {
					t.Errorf("Release should not have PR number since is an invalid PR number")
				}
				app, err = svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test-with-only-pr-number"})
				if err != nil {
					t.Errorf("Error fetching information for app test")
				}
				testApp = app.Application
				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber == "" {
					t.Errorf("Release should have PR number \"678\", but got %q", releases[0].PrNumber)
				}
				//Check Dev
				//Note that EnvironmentGroups are sorted, so it's dev,staging,production (see MapEnvironmentsToGroups for details on sorting)
				devGroup := resp.EnvironmentGroups[0]
				if devGroup.EnvironmentGroupName != "dev" {
					t.Errorf("dev environmentGroup has wrong name: %q", devGroup.EnvironmentGroupName)
				}
				dev := devGroup.Environments[0]
				if dev.Name != "development" {
					t.Errorf("development environment has wrong name: %q", dev.Name)
				}
				if dev.Config.Upstream == nil {
					t.Errorf("development environment has wrong upstream: %#v", dev.Config.Upstream)
				} else {
					if !dev.Config.Upstream.GetLatest() {
						t.Errorf("production environment has wrong upstream: %#v", dev.Config.Upstream)
					}
				}

				if len(dev.Locks) != 1 {
					t.Errorf("development environment has wrong locks: %#v", dev.Locks)
				}
				if lck, ok := dev.Locks["manual"]; !ok {
					t.Errorf("development environment doesn't contain manual lock: %#v", dev.Locks)
				} else {
					if lck.Message != "please" {
						t.Errorf("development environment manual lock has wrong message: %q", lck.Message)
					}
				}

				if _, ok := dev.TeamLocks["test-team"]; !ok {
					t.Errorf("development environment doesn't contain manual-team lock: %#v", dev.TeamLocks)

				}
				// check team lock
				if len(dev.TeamLocks["test-team"].Locks) != 1 {
					t.Errorf("development environment doesn't contain manual-team lock: %#v", dev.TeamLocks)
				} else {
					lck := dev.TeamLocks["test-team"].Locks[0]
					if lck.Message != "team lock message" {
						t.Errorf("development environment manual lock has wrong message: %q", lck.Message)
					}
				}

				// Check staging
				stageGroup := resp.EnvironmentGroups[1]
				if stageGroup.EnvironmentGroupName != "staging" {
					t.Errorf("staging environmentGroup has wrong name: %q", stageGroup.EnvironmentGroupName)
				}
				stage := stageGroup.Environments[0]
				if stage.Name != "staging" {
					t.Errorf("staging environment has wrong name: %q", stage.Name)
				}
				if stage.Config.Upstream == nil {
					t.Errorf("staging environment has wrong upstream: %#v", stage.Config.Upstream)
				} else {
					if stage.Config.Upstream.GetEnvironment() != "development" {
						t.Errorf("staging environment has wrong upstream: %#v", stage.Config.Upstream)
					}
					if stage.Config.Upstream.GetLatest() {
						t.Errorf("staging environment has wrong upstream: %#v", stage.Config.Upstream)
					}
				}
				if len(stage.Locks) != 0 {
					t.Errorf("staging environment has wrong locks: %#v", stage.Locks)
				}

				// Check production
				prodGroup := resp.EnvironmentGroups[2]
				if prodGroup.EnvironmentGroupName != "production" {
					t.Errorf("prod environmentGroup has wrong name: %q", prodGroup.EnvironmentGroupName)
				}
				prod := prodGroup.Environments[0]
				if prod.Name != "production" {
					t.Errorf("production environment has wrong name: %q", prod.Name)
				}
				if prod.Config.Upstream == nil {
					t.Errorf("production environment has wrong upstream: %#v", prod.Config.Upstream)
				} else {
					if prod.Config.Upstream.GetEnvironment() != "staging" {
						t.Errorf("production environment has wrong upstream: %#v", prod.Config.Upstream)
					}
					if prod.Config.Upstream.GetLatest() {
						t.Errorf("production environment has wrong upstream: %#v", prod.Config.Upstream)
					}
				}
				if len(prod.Locks) != 0 {
					t.Errorf("production environment has wrong locks: %#v", prod.Locks)
				}

				//Check applications
				if len(resp.LightweightApps) != 4 {
					t.Errorf("expected two application, got %#v", resp.LightweightApps)
				}

				app, err = svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test"})
				if err != nil {
					t.Errorf("Error fetching information for app test")
				}
				test := app.Application
				if _, ok := app.Deployments["development"]; !ok {
					t.Errorf("no deployments foubnd on development")
				}

				got := app.Deployments["development"].GetDeploymentMetaData().DeployAuthor
				if got != "test tester" {
					t.Errorf("development environment deployment did not create deploymentMetaData, got %s", got)
				}

				if app.Deployments["development"].Version != 1 {
					t.Errorf("test application has not version 1 but %d", app.Deployments["development"].Version)
				}
				if len(dev.AppLocks) != 0 {
					t.Errorf("test application has locks in development: %#v", dev.AppLocks)
				}

				if test.Name != "test" {
					t.Errorf("test applications name is not test but %q", test.Name)
				}
				if len(test.Releases) != 1 {
					t.Errorf("expected one release, got %#v", test.Releases)
				}
				if test.Releases[0].Version != 1 {
					t.Errorf("expected test release version to be 1, but got %d", test.Releases[0].Version)
				}
				if test.Releases[0].SourceAuthor != "example <example@example.com>" {
					t.Errorf("expected test source author to be \"example <example@example.com>\", but got %q", test.Releases[0].SourceAuthor)
				}
				if test.Releases[0].SourceMessage != "changed something (#678)" {
					t.Errorf("expected test source message to be \"changed something\", but got %q", test.Releases[0].SourceMessage)
				}
				if test.Releases[0].SourceCommitId != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
					t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", test.Releases[0].SourceCommitId)
				}

				app, err = svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: "test-with-team"})
				if err != nil {
					t.Errorf("Error fetching information for app test")
				}
				testWithTeam := app.Application

				if testWithTeam.Team != "test-team" {
					t.Errorf("application team is not test-team but %q", testWithTeam.Team)
				}

			},
		},
		{
			Name: "A stream overview works",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config:      config.EnvironmentConfig{},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "v1",
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"development": "v2",
					},
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
						t.Fatal(err)
					}
				}()

				// Check that we get a first overview
				overview1 := <-ch
				if overview1 == nil {
					t.Fatal("overview is nil")
				}
				v1 := overview1.GetEnvironmentGroups()[0].GetEnvironments()[0].GetLocks()

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
				v2 := overview2.GetEnvironmentGroups()[0].GetEnvironments()[0].GetLocks()
				if diff := cmp.Diff(v1, v2); diff == "" {
					t.Fatalf("Versions are not different: %q vs %q", v1, v2)
				}

				cancel()
				wg.Wait()
			},
		},
		{
			Name: "Test with DB",
			DB:   true,
			ExpectedCachedOverview: &api.GetOverviewResponse{
				EnvironmentGroups: []*api.EnvironmentGroup{
					{
						EnvironmentGroupName: "dev",
						Environments: []*api.Environment{
							{
								Name: "development",
								Config: &api.EnvironmentConfig{
									Upstream: &api.EnvironmentConfig_Upstream{
										Latest: &upstreamLatest,
									},
									Argocd: &api.EnvironmentConfig_ArgoCD{
										Destination: &api.EnvironmentConfig_ArgoCD_Destination{},
									},
									EnvironmentGroup: &dev,
								},
								Priority: api.Priority_YOLO,
							},
						},
						Priority: api.Priority_YOLO,
					},
				},
				LightweightApps: []*api.OverviewApplication{
					{
						Name: "test",
						Team: "team-123",
					},
				},
				GitRevision: "0",
			},
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
					Manifests: map[string]string{
						"dev": "v1",
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
					Manifests: map[string]string{
						"dev": "v2",
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
				if dev.Name != "development" {
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
				if test.Releases[1].Version != 1 {
					t.Errorf("expected test release version to be 1, but got %d", test.Releases[0].Version)
				}
				if test.Releases[1].SourceAuthor != "example <example@example.com>" {
					t.Errorf("expected test source author to be \"example <example@example.com>\", but got %q", test.Releases[0].SourceAuthor)
				}
				if test.Releases[1].SourceMessage != "changed something (#678)" {
					t.Errorf("expected test source message to be \"changed something\", but got %q", test.Releases[0].SourceMessage)
				}
				if test.Releases[1].SourceCommitId != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
					t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", test.Releases[0].SourceCommitId)
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
				migrationsPath, err := testutil.CreateMigrationsPath(4)
				if err != nil {
					t.Fatal(err)
				}
				dbConfig := &db.DBConfig{
					DriverName:     "sqlite3",
					MigrationsPath: migrationsPath,
					WriteEslOnly:   false,
				}
				repo, err = setupRepositoryTestWithDB(t, dbConfig)
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
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				DBHandler:  repo.State().DBHandler,
				Context:    context.Background(),
			}
			tc.Test(t, svc)
			if tc.DB {
				repo.State().DBHandler.WithTransaction(testutil.MakeTestContext(), false, func(ctx context.Context, transaction *sql.Tx) error {
					cachedResponse, err := repo.State().DBHandler.ReadLatestOverviewCache(ctx, transaction)
					if err != nil {
						return err
					}
					cachedResponse.GitRevision = "0"
					if diff := cmp.Diff(tc.ExpectedCachedOverview, cachedResponse, protocmp.Transform(), protocmp.IgnoreFields(&api.Release{}, "created_at"), protocmp.IgnoreFields(&api.Lock{}, "created_at")); diff != "" {
						t.Errorf("latest overview cache mismatch (-want +got):\n%s", diff)
					}
					return nil
				})
			}
			close(shutdown)
		})
	}
}

func TestGetApplicationDetails(t *testing.T) {
	var dev = "dev"
	var env = "development"
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
						},
					},
					Team: "team-123",
				},
				Deployments: map[string]*api.Deployment{
					env: {
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
						EnvironmentGroup: &dev,
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
					Manifests: map[string]string{
						env: "v1",
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
			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err = setupRepositoryTestWithDB(t, dbConfig)
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
			expectedDeployment := expected.Deployments[env]
			resultDeployment := resp.Deployments[env]

			if diff := cmp.Diff(expectedDeployment, resultDeployment, cmpopts.IgnoreUnexported(api.Deployment{}), cmpopts.IgnoreUnexported(api.Deployment_DeploymentMetaData{}), cmpopts.IgnoreFields(api.Deployment_DeploymentMetaData{}, "DeployTime")); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
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

func TestOverviewServiceFromCommit(t *testing.T) {
	type step struct {
		Transformer repository.Transformer
	}
	tcs := []struct {
		Name  string
		Steps []step
	}{
		{
			Name: "A simple overview works",
			Steps: []step{
				{
					Transformer: &repository.CreateEnvironment{
						Environment: "development",
						Config:      config.EnvironmentConfig{},
					},
				},
				{
					Transformer: &repository.CreateEnvironment{
						Environment: "staging",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Latest: true,
							},
						},
					},
				},
				{
					Transformer: &repository.CreateEnvironment{
						Environment: "production",
						Config: config.EnvironmentConfig{
							Upstream: &config.EnvironmentConfigUpstream{
								Environment: "staging",
							},
						},
					},
				},
				{
					Transformer: &repository.CreateApplicationVersion{
						Application: "test",
						Manifests: map[string]string{
							"development": "dev",
						},

						SourceAuthor:   "example <example@example.com>",
						SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:  "changed something (#678)",
					},
				},
				{
					Transformer: &repository.CreateApplicationVersion{
						Application: "test-with-team",
						Manifests: map[string]string{
							"development": "dev",
						},
						Team: "test-team",
					},
				},
				{
					Transformer: &repository.CreateApplicationVersion{
						Application: "test-with-incorrect-pr-number",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:   "example <example@example.com>",
						SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:  "changed something (#678",
					},
				},
				{
					Transformer: &repository.CreateApplicationVersion{
						Application: "test-with-only-pr-number",
						Manifests: map[string]string{
							"development": "dev",
						},
						SourceAuthor:   "example <example@example.com>",
						SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
						SourceMessage:  "(#678)",
					},
				},
				{
					Transformer: &repository.DeployApplicationVersion{
						Application: "test",
						Environment: "development",
						Version:     1,
					},
				},
				{
					Transformer: &repository.DeployApplicationVersion{
						Application: "test-with-team",
						Environment: "development",
						Version:     1,
					},
				},
				{
					Transformer: &repository.CreateEnvironmentLock{
						Environment: "development",
						LockId:      "manual",
						Message:     "please",
					},
				},
				{
					Transformer: &repository.CreateEnvironmentApplicationLock{
						Environment: "production",
						Application: "test",
						LockId:      "manual",
						Message:     "no",
					},
				},
				{
					Transformer: &repository.CreateEnvironmentTeamLock{
						Environment: "production",
						Team:        "test-team",
						LockId:      "manual",
						Message:     "no",
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatal(err)
			}
			svc := &OverviewServiceServer{
				Repository: repo,
				Shutdown:   shutdown,
				Context:    context.Background(),
			}

			ov, err := svc.GetOverview(testutil.MakeTestContext(), &api.GetOverviewRequest{})
			if err != nil {
				t.Errorf("expected no error, got %s", err)
			}
			if ov.GitRevision != "" {
				t.Errorf("expected git revision to be empty, got %q", ov.GitRevision)
			}
			revisions := map[string]*api.GetOverviewResponse{}
			for _, tr := range tc.Steps {
				if err := repo.Apply(testutil.MakeTestContext(), tr.Transformer); err != nil {
					t.Fatal(err)
				}
				ov, err = svc.GetOverview(testutil.MakeTestContext(), &api.GetOverviewRequest{})
				if err != nil {
					t.Errorf("expected no error, got %s", err)
				}
				if ov.GitRevision == "" {
					t.Errorf("expected git revision to be non-empty")
				}
				revisions[ov.GitRevision] = ov
			}
			for rev := range revisions {
				ov, err = svc.GetOverview(testutil.MakeTestContext(), &api.GetOverviewRequest{GitRevision: rev})
				if err != nil {
					t.Errorf("expected no error, got %s", err)
				}
				if ov.GitRevision != rev {
					t.Errorf("expected git revision to be %q, but got %q", rev, ov.GitRevision)
				}
			}
			close(shutdown)
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
			Name: "Update overview Deployment Attempt",
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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
							},
							{
								Version:        1,
								SourceCommitId: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
								SourceAuthor:   "example <example@example.com>",
								SourceMessage:  "changed something (#678)",
								PrNumber:       "678",
								CreatedAt:      &timestamppb.Timestamp{Seconds: 1, Nanos: 1},
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

			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err = setupRepositoryTestWithDB(t, dbConfig)
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
				Context:    context.Background(),
			}
			for _, currentAppName := range tc.AppsNamesToCheck {
				response, err := svc.GetAppDetails(ctx, &api.GetAppDetailsRequest{AppName: currentAppName})

				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(tc.ExpectedAppDetails[currentAppName], response, getAppDetailsIgnoredTypes(), cmpopts.IgnoreFields(api.Release{}, "CreatedAt"), cmpopts.IgnoreFields(api.Deployment_DeploymentMetaData{}, "DeployTime"), cmpopts.IgnoreFields(api.Lock{}, "CreatedAt")); diff != "" {
					t.Errorf("response missmatch (-want, +got): %s\n", diff)
				}
			}
		})

	}
}

func TestCalculateWarnings(t *testing.T) {
	var dev = "dev"
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
						EnvironmentGroup: &dev,
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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
					Manifests: map[string]string{
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

			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err = setupRepositoryTestWithDB(t, dbConfig)
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
				Context:    context.Background(),
			}

			for _, appName := range tc.AppsNamesToCheck {
				appDetails, err := svc.GetAppDetails(testutil.MakeTestContext(), &api.GetAppDetailsRequest{AppName: appName})
				if err != nil {
					t.Error(err)
				}

				if diff := cmp.Diff(tc.ExpectedWarnings[appName], appDetails.Application.Warnings, getAppDetailsIgnoredTypes()); diff != "" {
					t.Errorf("response missmatch (-want, +got): %s\n", diff)
				}
			}
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
