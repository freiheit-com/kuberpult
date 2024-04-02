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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"sync"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
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

func makeApps(apps ...*api.Environment_Application) map[string]*api.Environment_Application {
	var result map[string]*api.Environment_Application = map[string]*api.Environment_Application{}
	for i := 0; i < len(apps); i++ {
		app := apps[i]
		result[app.Name] = app
	}
	return result
}

func makeEnv(envName string, groupName string, upstream *api.EnvironmentConfig_Upstream, apps map[string]*api.Environment_Application) *api.Environment {
	return &api.Environment{
		Name: envName,
		Config: &api.EnvironmentConfig{
			Upstream:         upstream,
			EnvironmentGroup: &groupName,
		},
		Locks:              map[string]*api.Lock{},
		Applications:       apps,
		DistanceToUpstream: 0,
		Priority:           api.Priority_UPSTREAM, // we are 1 away from prod, hence pre-prod
	}
}

func makeApp(appName string, version uint64) *api.Environment_Application {
	return &api.Environment_Application{
		Name:            appName,
		Version:         version,
		Locks:           nil,
		QueuedVersion:   0,
		UndeployVersion: false,
		ArgoCd:          nil,
	}
}
func makeEnvGroup(envGroupName string, environments []*api.Environment) *api.EnvironmentGroup {
	return &api.EnvironmentGroup{
		EnvironmentGroupName: envGroupName,
		Environments:         environments,
		DistanceToUpstream:   0,
	}
}

func makeUpstreamLatest() *api.EnvironmentConfig_Upstream {
	f := true
	return &api.EnvironmentConfig_Upstream{
		Latest: &f,
	}
}

func makeUpstreamEnv(upstream string) *api.EnvironmentConfig_Upstream {
	return &api.EnvironmentConfig_Upstream{
		Environment: &upstream,
	}
}

func TestCalculateWarnings(t *testing.T) {
	var dev = "dev"
	tcs := []struct {
		Name             string
		AppName          string
		Groups           []*api.EnvironmentGroup
		ExpectedWarnings []*api.Warning
	}{
		{
			Name:    "no envs - no warning",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev-de", dev, makeUpstreamLatest(), nil),
				})},
			ExpectedWarnings: []*api.Warning{},
		},
		{
			Name:    "app deployed in higher version on upstream should warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 2))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps(makeApp("foo", 1))),
				}),
			},
			ExpectedWarnings: []*api.Warning{
				{
					WarningType: &api.Warning_UnusualDeploymentOrder{
						UnusualDeploymentOrder: &api.UnusualDeploymentOrder{
							UpstreamVersion:     1,
							UpstreamEnvironment: "dev",
							ThisVersion:         2,
							ThisEnvironment:     "prod",
						},
					},
				},
			},
		},
		{
			Name:    "app deployed in same version on upstream should not warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 2))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps(makeApp("foo", 2))),
				}),
			},
			ExpectedWarnings: []*api.Warning{},
		},
		{
			Name:    "app deployed in no version on upstream should warn",
			AppName: "foo",
			Groups: []*api.EnvironmentGroup{
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("prod", dev, makeUpstreamEnv("dev"),
						makeApps(makeApp("foo", 1))),
				}),
				makeEnvGroup(dev, []*api.Environment{
					makeEnv("dev", dev, makeUpstreamLatest(),
						makeApps()),
				}),
			},
			ExpectedWarnings: []*api.Warning{
				{
					WarningType: &api.Warning_UpstreamNotDeployed{
						UpstreamNotDeployed: &api.UpstreamNotDeployed{
							UpstreamEnvironment: "dev",
							ThisVersion:         1,
							ThisEnvironment:     "prod",
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			actualWarnings := CalculateWarnings(testutil.MakeTestContext(), tc.AppName, tc.Groups)
			if len(actualWarnings) != len(tc.ExpectedWarnings) {
				t.Errorf("Different number of warnings. got: %s\nwant: %s", actualWarnings, tc.ExpectedWarnings)
			}
			for i := 0; i < len(actualWarnings); i++ {
				actualWarning := actualWarnings[i]
				expectedWarning := tc.ExpectedWarnings[i]
				if diff := cmp.Diff(actualWarning.String(), expectedWarning.String()); diff != "" {
					t.Errorf("Different warning at index [%d]:\ngot:  %s\nwant: %s", i, actualWarning, expectedWarning)
				}
			}
		})
	}

}

func TestOverviewService(t *testing.T) {
	var dev = "dev"
	tcs := []struct {
		Name  string
		Setup []repository.Transformer
		Test  func(t *testing.T, svc *OverviewServiceServer)
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
					SourceRepoUrl:  "testing@testing.com/abc",
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
					SourceRepoUrl:  "testing@testing.com/abc",
				},
				&repository.CreateApplicationVersion{
					Application: "test-with-only-pr-number",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceAuthor:   "example <example@example.com>",
					SourceCommitId: "deadbeef",
					SourceMessage:  "(#678)",
					SourceRepoUrl:  "testing@testing.com/abc",
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
				//&repository.CreateEnvironmentTeamLock{
				//	Environment: "development",
				//	Team:        "test-team",
				//	LockId:      "manual",
				//	Message:     "no",
				//},
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
				if testApp.SourceRepoUrl != "testing@testing.com/abc" {
					t.Errorf("Expected \"testing@testing.com/abc\", but got %#q", resp.Applications["test"].SourceRepoUrl)
				}
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
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			shutdown := make(chan struct{}, 1)
			repo, err := setupRepositoryTest(t)
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
			}
			tc.Test(t, svc)
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
						SourceRepoUrl:  "testing@testing.com/abc",
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
						SourceRepoUrl:  "testing@testing.com/abc",
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
						SourceRepoUrl:  "testing@testing.com/abc",
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

func groupFromEnvs(environments []*api.Environment) []*api.EnvironmentGroup {
	return []*api.EnvironmentGroup{
		{
			EnvironmentGroupName: "group1",
			Environments:         environments,
		},
	}
}

func TestDeriveUndeploySummary(t *testing.T) {
	var tcs = []struct {
		Name           string
		AppName        string
		groups         []*api.EnvironmentGroup
		ExpectedResult api.UndeploySummary
	}{
		{
			Name:           "No Environments",
			AppName:        "foo",
			groups:         []*api.EnvironmentGroup{},
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "one Environment but no Application",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"bar": { // different app
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with undeploy",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "One Env with normal version",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all undeploy",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_UNDEPLOY,
		},
		{
			Name:    "Two Envs all normal",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_NORMAL,
		},
		{
			Name:    "Two Envs all different",
			AppName: "foo",
			groups: groupFromEnvs([]*api.Environment{
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: true,
							Version:         666,
						},
					},
				},
				{
					Applications: map[string]*api.Environment_Application{
						"foo": {
							UndeployVersion: false,
							Version:         666,
						},
					},
				},
			}),
			ExpectedResult: api.UndeploySummary_MIXED,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			actualResult := deriveUndeploySummary(tc.AppName, tc.groups)
			if !cmp.Equal(tc.ExpectedResult, actualResult) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedResult, actualResult))
			}
		})
	}
}
