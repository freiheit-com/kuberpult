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
	"crypto/md5"
	"fmt"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

func TestVersion(t *testing.T) {
	type expectedVersion struct {
		Environment            string
		Application            string
		ExpectedVersion        uint64
		ExpectedDeployedAt     time.Time
		ExpectedSourceCommitId string
	}
	tcs := []struct {
		Name             string
		Setup            []repository.Transformer
		ExpectedVersions []expectedVersion
	}{
		{
			Name: "simple-tests",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:        "development",
					Application:        "test",
					ExpectedVersion:    1,
					ExpectedDeployedAt: time.Unix(2, 0),
				},
				{
					Environment:     "staging",
					Application:     "test",
					ExpectedVersion: 0,
				},
			},
		},
		{
			Name: "with source commits",
			Setup: []repository.Transformer{
				&repository.CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceCommitId: "deadbeef",
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:            "development",
					Application:            "test",
					ExpectedVersion:        1,
					ExpectedDeployedAt:     time.Unix(2, 0),
					ExpectedSourceCommitId: "deadbeef",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			sv := &VersionServiceServer{Repository: repo}

			for i, transformer := range tc.Setup {
				now := time.Unix(int64(i), 0)
				ctx := repository.WithTimeNow(testutil.MakeTestContext(), now)
				err := repo.Apply(ctx, transformer)
				if err != nil {
					t.Fatal(err)
				}
			}
			cid := repo.State().Commit.Id().String()
			for _, ev := range tc.ExpectedVersions {
				res, err := sv.GetVersion(context.Background(), &api.GetVersionRequest{
					GitRevision: cid,
					Application: ev.Application,
					Environment: ev.Environment,
				})
				if err != nil {
					t.Fatal(err)
				}
				if res.Version != ev.ExpectedVersion {
					t.Errorf("got wrong version for %s/%s: expected %d but got %d", ev.Application, ev.Environment, ev.ExpectedVersion, res.Version)
				}
				if ev.ExpectedDeployedAt.IsZero() {
					if res.DeployedAt != nil {
						t.Errorf("got wrong deployed at for %s/%s: expected <nil> but got %q", ev.Application, ev.Environment, res.DeployedAt)
					}
				} else {
					if !res.DeployedAt.AsTime().Equal(ev.ExpectedDeployedAt) {
						t.Errorf("got wrong deployed at for %s/%s: expected %q but got %q", ev.Application, ev.Environment, ev.ExpectedDeployedAt, res.DeployedAt.AsTime())
					}
				}
				if ev.ExpectedSourceCommitId != res.SourceCommitId {
					t.Errorf("go wrong source commit id, expected %q, got %q", ev.ExpectedSourceCommitId, res.SourceCommitId)
				}
			}
		})
	}
}

func TestGetManifests(t *testing.T) {
	type testCase struct {
		name    string
		setup   []repository.Transformer
		req     *api.GetManifestsRequest
		want    *api.GetManifestsResponse
		wantErr error
	}

	appName := "app-default"
	appNameOther := "app-other"
	fixtureRequest := func(mods ...func(*api.GetManifestsRequest)) *api.GetManifestsRequest {
		req := &api.GetManifestsRequest{
			Application: appName,
			Release:     "latest",
		}
		for _, m := range mods {
			m(req)
		}
		return req
	}
	fixtureSetupEnv := func() []repository.Transformer {
		return []repository.Transformer{
			&repository.CreateEnvironment{
				Environment: "development",
				Config: config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
			},
			&repository.CreateEnvironment{
				Environment: "staging",
				Config: config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
			},
		}
	}
	fixtureRelease := func(application string, release uint64) *repository.CreateApplicationVersion {
		return &repository.CreateApplicationVersion{
			Application: application,
			Version:     release,
			Manifests: map[string]string{
				"development": fmt.Sprintf("dev-manifest for %s in release %d", application, release),
				"staging":     fmt.Sprintf("staging-manifest for %s in release %d", application, release),
			},
			SourceCommitId: fmt.Sprintf("%x",
				md5.Sum(
					[]byte(
						fmt.Sprintf("source commit id for app %s and release %d", application, release),
					),
				),
			),
		}
	}

	fixtureReleaseToManifests := func(release *repository.CreateApplicationVersion) *api.GetManifestsResponse {
		return &api.GetManifestsResponse{
			Release: &api.Release{
				Version:        release.Version,
				SourceCommitId: release.SourceCommitId,
			},
			Manifests: map[string]*api.Manifest{
				"development": {
					Environment: "development",
					Content:     release.Manifests["development"],
				},
				"staging": {
					Environment: "staging",
					Content:     release.Manifests["staging"],
				},
			},
		}
	}

	for _, tc := range []*testCase{
		func() *testCase {
			release := fixtureRelease(appName, 3)

			return &testCase{
				name: "happy path",
				setup: append(fixtureSetupEnv(),
					fixtureRelease(appNameOther, 1),
					fixtureRelease(appNameOther, 2),
					fixtureRelease(appName, 1),
					fixtureRelease(appName, 2),
					release,
				),
				req:  fixtureRequest(),
				want: fixtureReleaseToManifests(release),
			}
		}(),
		func() *testCase {
			release := fixtureRelease(appName, 2)

			return &testCase{
				name: "request specific release",
				setup: append(fixtureSetupEnv(),
					fixtureRelease(appName, 1),
					fixtureRelease(appNameOther, 1),
					release,
					fixtureRelease(appName, 3),
					fixtureRelease(appNameOther, 2),
				),
				req:  fixtureRequest(func(req *api.GetManifestsRequest) { req.Release = "2" }),
				want: fixtureReleaseToManifests(release),
			}
		}(),
		{
			name: "no release specified",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appName, 1),
				fixtureRelease(appName, 2),
				fixtureRelease(appName, 3),
			),
			req:     fixtureRequest(func(req *api.GetManifestsRequest) { req.Release = "" }),
			wantErr: status.Error(codes.InvalidArgument, "invalid release number, expected uint or 'latest'"),
		},
		{
			name: "no application specified",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appName, 1),
				fixtureRelease(appName, 2),
				fixtureRelease(appName, 3),
			),
			req:     fixtureRequest(func(req *api.GetManifestsRequest) { req.Application = "" }),
			wantErr: status.Error(codes.InvalidArgument, "no application specified"),
		},
		{
			name: "no releases for application",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appNameOther, 1),
				fixtureRelease(appNameOther, 2),
				fixtureRelease(appNameOther, 3),
			),
			req:     fixtureRequest(),
			wantErr: status.Errorf(codes.NotFound, "no releases found for application %s", appName),
		},
	} {
		tc := tc // TODO SRX-SRRONB: Remove after switching to go v1.22
		t.Run(tc.name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			sv := &VersionServiceServer{Repository: repo}

			for i, transformer := range tc.setup {
				now := time.Unix(int64(i), 0)
				ctx := repository.WithTimeNow(testutil.MakeTestContext(), now)
				err := repo.Apply(ctx, transformer)
				if err != nil {
					t.Fatal(err)
				}
			}

			got, err := sv.GetManifests(context.Background(), tc.req)
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.want, got, protocmp.Transform(), protocmp.IgnoreFields(&api.Release{}, "created_at")); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
