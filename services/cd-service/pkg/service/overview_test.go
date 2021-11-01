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
	"os/exec"
	"path"
	"sync"
	"testing"

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
					SourceMessage:  "changed something",
				},
				&repository.DeployApplicationVersion{
					Application: "test",
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
				if len(dev.Applications) != 1 {
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
					if app.VersionCommit == nil {
						t.Errorf("test application in dev has no version commit")
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
					if app.VersionCommit != nil {
						t.Errorf("version commit in production is not nil")
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
					if test.Releases[0].SourceMessage != "changed something" {
						t.Errorf("expected test source message to be \"changed something\", but got %q", test.Releases[0].SourceMessage)
					}
					if test.Releases[0].SourceCommitId != "deadbeef" {
						t.Errorf("expected test source commit id to be \"deadbeef\", but got %q", test.Releases[0].SourceCommitId)
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
			t.Parallel()
			dir := t.TempDir()
			remoteDir := path.Join(dir, "remote")
			localDir := path.Join(dir, "local")
			cmd := exec.Command("git", "init", "--bare", remoteDir)
			cmd.Start()
			cmd.Wait()
			repo, err := repository.NewWait(
				context.Background(),
				repository.Config{
					URL:            remoteDir,
					Path:           localDir,
					CommitterEmail: "kuberpult@freiheit.com",
					CommitterName:  "kuberpult",
				},
			)
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
			}
			tc.Test(t, svc)
		})
	}
}
