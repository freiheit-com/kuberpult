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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
)

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
			SourceCommitId: "",
		}
	}

	fixtureReleaseToManifests := func(release *repository.CreateApplicationVersion) *api.GetManifestsResponse {
		return &api.GetManifestsResponse{
			Release: &api.Release{
				Version:        release.Version,
				SourceCommitId: release.SourceCommitId,
				Environments:   []string{"development", "staging"},
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
			migrationsPath, err := testutil.CreateMigrationsPath(4)
			if err != nil {
				t.Fatal(err)
			}
			dbConfig := &db.DBConfig{
				DriverName:     "sqlite3",
				MigrationsPath: migrationsPath,
				WriteEslOnly:   false,
			}
			repo, err := setupRepositoryTestWithDB(t, dbConfig)
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			ctx := testutil.MakeTestContext()
			sv := &VersionServiceServer{Repository: repo}

			_ = repo.State().DBHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.setup...)
				if err2 != nil {
					return err2
				}

				return nil
			})
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
