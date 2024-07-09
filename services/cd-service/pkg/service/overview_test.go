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
	"encoding/json"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"sync"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
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
	tcs := []struct {
		Name         string
		Setup        []repository.Transformer
		Test         func(t *testing.T, svc *OverviewServiceServer)
		DB           bool
		ExpectedBlob string
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
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeef",
					SourceMessage:  "changed something (#678)",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-team",
					Manifests: map[string]string{
						"development": "dev",
					},
					Team: "test-team",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-incorrect-pr-number",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeef",
					SourceMessage:  "changed something (#678",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-only-pr-number",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeef",
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

				const expectedEnvs = 3
				if len(resp.EnvironmentGroups) != expectedEnvs {
					t.Errorf("expected %d environmentGroups, got %q", expectedEnvs, resp.EnvironmentGroups)
				}
				testApp := resp.Applications["test"]
				releases := testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "678" {
					t.Errorf("Release should have PR number \"678\", but got %q", releases[0].PrNumber)
				}
				testApp = resp.Applications["test-with-team"]
				if testApp.SourceRepoUrl != "" {
					t.Errorf("Expected \"\", but got %#q", resp.Applications["test"].SourceRepoUrl)
				}
				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "" {
					t.Errorf("Release should not have PR number")
				}
				testApp = resp.Applications["test-with-incorrect-pr-number"]
				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber != "" {
					t.Errorf("Release should not have PR number since is an invalid PR number")
				}
				testApp = resp.Applications["test-with-only-pr-number"]
				releases = testApp.Releases
				if len(releases) != 1 {
					t.Errorf("Expected one release, but got %#q", len(releases))
				}
				if releases[0].PrNumber == "" {
					t.Errorf("Release should have PR number \"678\", but got %q", releases[0].PrNumber)
				}
				// Check Dev
				// Note that EnvironmentGroups are sorted, so it's dev,staging,production (see MapEnvironmentsToGroups for details on sorting)
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

				// check team lock
				if lck, ok := dev.Applications["test-with-team"].TeamLocks["manual-team-lock"]; !ok {
					t.Errorf("development environment doesn't contain manual-team lock: %#v", dev.Locks)
				} else {
					if lck.Message != "team lock message" {
						t.Errorf("development environment manual lock has wrong message: %q", lck.Message)
					}
				}
				if len(dev.Applications) != 4 {
					t.Errorf("development environment has wrong applications: %#v", dev.Applications)
				}
				if app, ok := dev.Applications["test"]; !ok {
					t.Errorf("development environment has wrong applications: %#v", dev.Applications)
				} else {
					if app.Version != 1 {
						t.Errorf("test application has not version 1 but %d", app.Version)
					}
					if len(app.Locks) != 0 {
						t.Errorf("test application has locks in development: %#v", app.Locks)
					}
				}

				got := dev.Applications["test"].GetDeploymentMetaData().DeployAuthor
				if got != "test tester" {
					t.Errorf("development environment deployment did not create deploymentMetaData, got %s", got)
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
				if len(stage.Applications) != 0 {
					t.Errorf("staging environment has wrong applications: %#v", stage.Applications)
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
				if len(prod.Applications) != 1 {
					t.Errorf("production environment has wrong applications: %#v", prod.Applications)
				}
				if app, ok := prod.Applications["test"]; !ok {
					t.Errorf("production environment has wrong applications: %#v", prod.Applications)
				} else {
					if app.Version != 0 {
						t.Errorf("test application has not version 0 but %d", app.Version)
					}
					if len(app.Locks) != 1 {
						t.Errorf("test application has locks in production: %#v", app.Locks)
					}
				}

				// Check applications
				if len(resp.Applications) != 4 {
					t.Errorf("expected two application, got %#v", resp.Applications)
				}
				if test, ok := resp.Applications["test"]; !ok {
					t.Errorf("test application is missing in %#v", resp.Applications)
				} else {
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
					if test.Releases[0].SourceCommitId != "deadbeef" {
						t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", test.Releases[0].SourceCommitId)
					}
				}
				if testWithTeam, ok := resp.Applications["test-with-team"]; !ok {
					t.Errorf("test-with-team application is missing in %#v", resp.Applications)
				} else {
					if testWithTeam.Team != "test-team" {
						t.Errorf("application team is not test-team but %q", testWithTeam.Team)
					}
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
				v1 := overview1.GetEnvironmentGroups()[0].GetEnvironments()[0].GetApplications()["test"].Version

				// Update a version and see that the version changed
				err := svc.Repository.Apply(ctx, &repository.DeployApplicationVersion{
					Application: "test",
					Environment: "development",
					Version:     2,
				})
				if err != nil {
					t.Fatal(err)
				}

				// Check that the second overview is different
				overview2 := <-ch
				if overview2 == nil {
					t.Fatal("overview is nil")
				}
				v2 := overview2.EnvironmentGroups[0].Environments[0].Applications["test"].Version
				if v1 == v2 {
					t.Fatalf("Versions are not different: %q vs %q", v1, v2)
				}

				if overview1.GitRevision == overview2.GitRevision {
					t.Errorf("Git Revisions are not different: %q", overview1.GitRevision)
				}

				cancel()
				wg.Wait()
			},
		},
		{
			Name:         "Test with DB",
			DB:           true,
			ExpectedBlob: "{\"applications\":{\"test\":{\"name\":\"test\",\"releases\":[{\"version\":1,\"source_commit_id\":\"deadbeef\",\"source_author\":\"example \\u003cexample@example.com\\u003e\",\"source_message\":\"changed something (#678)\",\"created_at\":{\"seconds\":1,\"nanos\":1},\"pr_number\":\"678\"}],\"team\":\"team-123\"}},\"environment_groups\":[{\"environment_group_name\":\"dev\",\"environments\":[{\"name\":\"development\",\"config\":{\"upstream\":{\"latest\":true},\"argocd\":{},\"environment_group\":\"dev\"},\"applications\":{\"test\":{\"name\":\"test\",\"version\":1,\"deployment_meta_data\":{\"deploy_author\":\"testmail@example.com\",\"deploy_time\":\"1\"},\"team\":\"team-123\"}},\"priority\":5}],\"priority\":5}],\"git_revision\":\"test_git_revision\"}",
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
					Authentication:   repository.Authentication{},
					Version:          1,
					SourceCommitId:   "deadbeef",
					SourceAuthor:     "example <example@example.com>",
					SourceMessage:    "changed something (#678)",
					Team:             "team-123",
					DisplayVersion:   "",
					WriteCommitData:  false,
					PreviousCommit:   "",
					TransformerEslID: 1,
					Application:      "test",
					Manifests: map[string]string{
						"development": "v1",
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
				if len(resp.Applications) != 1 {
					t.Errorf("expected one application, got %#v", resp.Applications)
				}
				if test, ok := resp.Applications["test"]; !ok {
					t.Errorf("test application is missing in %#v", resp.Applications)
				} else {
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
					if test.Releases[0].SourceCommitId != "deadbeef" {
						t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", test.Releases[0].SourceCommitId)
					}
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
			}
			tc.Test(t, svc)
			if tc.DB {
				repo.State().DBHandler.WithTransaction(testutil.MakeTestContext(), false, func(ctx context.Context, transaction *sql.Tx) error {
					latestOverviewCache, err := repo.State().DBHandler.ReadLatestOverviewCache(ctx, transaction)
					if err != nil {
						return err
					}
					var cachedResponse *api.GetOverviewResponse
					if err := json.Unmarshal([]byte(latestOverviewCache.Blob), &cachedResponse); err != nil {
						return err
					}
					cachedResponse.GitRevision = "test_git_revision"
					cachedResponse.EnvironmentGroups[0].Environments[0].Applications["test"].DeploymentMetaData.DeployTime = "1"
					cachedResponse.Applications["test"].Releases[0].CreatedAt.Seconds = 1
					cachedResponse.Applications["test"].Releases[0].CreatedAt.Nanos = 1
					actualCachedResponse, err := json.Marshal(cachedResponse)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(tc.ExpectedBlob, string(actualCachedResponse)); diff != "" {
						t.Errorf("latest overview cache mismatch (-want +got):\n%s", diff)
					}
					return nil
				})
			}
			close(shutdown)
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
						SourceCommitId: "deadbeef",
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
						SourceCommitId: "deadbeef",
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
						SourceCommitId: "deadbeef",
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
				if revisions[ov.GitRevision] != nil {
					t.Errorf("git revision was observed twice: %q", ov.GitRevision)
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
