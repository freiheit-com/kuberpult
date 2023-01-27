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
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
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

type mockOverviewService_StreamDeployedOverviewServer struct {
	grpc.ServerStream
	Results chan *api.GetDeployedOverviewResponse
	Ctx     context.Context
}

func (m *mockOverviewService_StreamDeployedOverviewServer) Send(msg *api.GetDeployedOverviewResponse) error {
	m.Results <- msg
	return nil
}

func (m *mockOverviewService_StreamDeployedOverviewServer) Context() context.Context {
	return m.Ctx
}

func TestOverviewService(t *testing.T) {
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
					Config:      config.EnvironmentConfig{},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
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
			},
			Test: func(t *testing.T, svc *OverviewServiceServer) {
				resp, err := svc.GetOverview(context.Background(), &api.GetOverviewRequest{})
				if err != nil {
					t.Fatal(err)
				}
				if len(resp.Environments) != 3 {
					t.Errorf("expected three environments, got %q", resp.Environments)
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
				dev, ok := resp.Environments["development"]
				if !ok {
					t.Error("development environment not returned")
				}
				if dev.Name != "development" {
					t.Errorf("development environment has wrong name: %q", dev.Name)
				}
				if dev.Config.Upstream != nil {
					t.Errorf("development environment has wrong upstream: %#v", dev.Config.Upstream)
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
				if len(dev.Applications) != 2 {
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

				// Check staging
				stage, ok := resp.Environments["staging"]
				if !ok {
					t.Error("staging environment not returned")
				}
				if stage.Name != "staging" {
					t.Errorf("staging environment has wrong name: %q", stage.Name)
				}
				if stage.Config.Upstream == nil {
					t.Errorf("staging environment has wrong upstream: %#v", stage.Config.Upstream)
				} else {
					if stage.Config.Upstream.GetEnvironment() != "" {
						t.Errorf("staging environment has wrong upstream: %#v", stage.Config.Upstream)
					}
					if !stage.Config.Upstream.GetLatest() {
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
				prod, ok := resp.Environments["production"]
				if !ok {
					t.Error("production environment not returned")
				}
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
				ctx, cancel := context.WithCancel(context.Background())
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
				v1 := overview1.GetEnvironments()["development"].GetApplications()["test"].Version

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
				v2 := overview2.Environments["development"].Applications["test"].Version
				if v1 == v2 {
					t.Fatalf("Versions are not different: %q vs %q", v1, v2)
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
				if err := repo.Apply(context.Background(), tr); err != nil {
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

func makeUpstreamLatest() *api.Environment_Config_Upstream {
	return &api.Environment_Config_Upstream{
		Upstream: &api.Environment_Config_Upstream_Latest{
			Latest: true,
		},
	}
}

func makeUpstreamEnvironment(env string) *api.Environment_Config_Upstream {
	return &api.Environment_Config_Upstream{
		Upstream: &api.Environment_Config_Upstream_Environment{
			Environment: env,
		},
	}
}

var nameStagingDe = "staging-de"
var nameDevDe = "dev-de"
var nameProdDe = "prod-de"
var nameWhoKnowsDe = "whoknows-de"

var nameStagingFr = "staging-fr"
var nameDevFr = "dev-fr"
var nameProdFr = "prod-fr"
var nameWhoKnowsFr = "whoknows-fr"

var nameStaging = "staging"
var nameDev = "dev"
var nameProd = "prod"
var nameWhoKnows = "whoknows"

func makeEnv(envName string, groupName string, upstream *api.Environment_Config_Upstream, distanceToUpstream uint32, priority api.Priority) *api.Environment {
	return &api.Environment{
		Name: envName,
		Config: &api.Environment_Config{
			Upstream:         upstream,
			EnvironmentGroup: &groupName,
		},
		Locks:              map[string]*api.Lock{},
		Applications:       map[string]*api.Environment_Application{},
		DistanceToUpstream: distanceToUpstream,
		Priority:           priority, // we are 1 away from prod, hence pre-prod
	}
}

func TestMapEnvironmentsToGroup(t *testing.T) {
	tcs := []struct {
		Name           string
		InputEnvs      map[string]config.EnvironmentConfig
		ExpectedResult []*api.EnvironmentGroup
	}{
		{
			Name: "One Environment is one Group",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: "",
						Latest:      true,
					},
					ArgoCd:           nil,
					EnvironmentGroup: &nameDevDe,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_PROD),
					},
					DistanceToUpstream: 0,
				},
			},
		},
		{
			Name: "Two Environments are two Groups",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PROD),
					},
					DistanceToUpstream: 1,
				},
			},
		},
		{
			// note that this is not a realistic example, we just want to make sure it does not crash!
			// some outputs may be nonsensical (like distanceToUpstream), but that's fine as long as it's stable!
			Name: "Two Environments with a loop",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamEnvironment(nameStagingDe), 4, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 4,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 5, api.Priority_PROD),
					},
					DistanceToUpstream: 5,
				},
			},
		},
		{
			Name: "Three Environments are three Groups",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					ArgoCd: nil,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					ArgoCd: nil,
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					ArgoCd: nil,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProdDe,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
				},
			},
		},
		{
			Name: "Four Environments in a row to ensure that Priority_UPSTREAM works",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
				},
				nameWhoKnowsDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameProdDe,
					},
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDevDe,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDevDe, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStagingDe,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStagingDe, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_OTHER),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProdDe,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProdDe, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 2,
				},
				{
					EnvironmentGroupName: nameWhoKnowsDe,
					Environments: []*api.Environment{
						makeEnv(nameWhoKnowsDe, nameWhoKnowsDe, makeUpstreamEnvironment(nameProdDe), 3, api.Priority_PROD),
					},
					DistanceToUpstream: 3,
				},
			},
		},
		{
			// this is a realistic example
			Name: "Three Groups with 2 envs each",
			InputEnvs: map[string]config.EnvironmentConfig{
				nameDevDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: &nameDev,
				},
				nameDevFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
					EnvironmentGroup: &nameDev,
				},
				nameStagingDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevDe,
					},
					EnvironmentGroup: &nameStaging,
				},
				nameStagingFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameDevFr,
					},
					EnvironmentGroup: &nameStaging,
				},
				nameProdDe: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingDe,
					},
					EnvironmentGroup: &nameProd,
				},
				nameProdFr: {
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: nameStagingFr,
					},
					EnvironmentGroup: &nameProd,
				},
			},
			ExpectedResult: []*api.EnvironmentGroup{
				{
					EnvironmentGroupName: nameDev,
					Environments: []*api.Environment{
						makeEnv(nameDevDe, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
						makeEnv(nameDevFr, nameDev, makeUpstreamLatest(), 0, api.Priority_UPSTREAM),
					},
					DistanceToUpstream: 0,
				},
				{
					EnvironmentGroupName: nameStaging,
					Environments: []*api.Environment{
						makeEnv(nameStagingDe, nameStaging, makeUpstreamEnvironment(nameDevDe), 1, api.Priority_PRE_PROD),
						makeEnv(nameStagingFr, nameStaging, makeUpstreamEnvironment(nameDevFr), 1, api.Priority_PRE_PROD),
					},
					DistanceToUpstream: 1,
				},
				{
					EnvironmentGroupName: nameProd,
					Environments: []*api.Environment{
						makeEnv(nameProdDe, nameProd, makeUpstreamEnvironment(nameStagingDe), 2, api.Priority_PROD),
						makeEnv(nameProdFr, nameProd, makeUpstreamEnvironment(nameStagingFr), 2, api.Priority_PROD),
					},
					DistanceToUpstream: 2,
				},
			},
		},
	}
	for _, tc := range tcs {
		opts := cmpopts.IgnoreUnexported(api.EnvironmentGroup{}, api.Environment{}, api.Environment_Config{}, api.Environment_Config_Upstream{})
		t.Run(tc.Name, func(t *testing.T) {
			actualResult := mapEnvironmentsToGroups(tc.InputEnvs)
			if !cmp.Equal(tc.ExpectedResult, actualResult, opts) {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedResult, actualResult, opts))
			}
		})
	}
}
