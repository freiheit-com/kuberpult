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
	"database/sql"
	"fmt"
	"os/exec"
	"path"
	"testing"
	gotime "time"

	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

func setupRepositoryTestWithPath(t *testing.T) (repository.Repository, string) {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &db.DBConfig{
		MigrationsPath: migrationsPath,
		DriverName:     "sqlite3",
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Errorf("could not start git init")
		return nil, ""
	}
	err = cmd.Wait()
	if err != nil {
		t.Errorf("could not wait for git init to finish")
		return nil, ""
	}

	repoCfg := repository.RepositoryConfig{
		URL:                 remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
		ReleaseVersionLimit: 2,
	}

	if dbConfig != nil {
		dbConfig.DbHost = dir

		migErr := db.RunDBMigrations(ctx, *dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		dbHandler, err := db.Connect(ctx, *dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = dbHandler
	}

	repo, err := repository.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, remoteDir
}

func TestVersion(t *testing.T) {
	type expectedVersion struct {
		Environment            string
		Application            string
		ExpectedVersion        uint64
		ExpectedDeployedAt     gotime.Time
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
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Manifests: map[string]string{
						"development": "dev",
					},
					Team:                "team-123",
					WriteCommitData:     false,
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:     "development",
					Application:     "test",
					ExpectedVersion: 1,
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
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&repository.CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Latest: true,
						},
					},
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
				&repository.CreateApplicationVersion{
					Application: "test",
					Version:     1,
					Team:        "team-123",
					Manifests: map[string]string{
						"development": "dev",
					},
					SourceCommitId:      "deadbeef",
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "testAuthorName", AuthorEmail: "testAuthorEmail@example.com"},
				},
			},
			ExpectedVersions: []expectedVersion{
				{
					Environment:            "development",
					Application:            "test",
					ExpectedVersion:        1,
					ExpectedDeployedAt:     gotime.Unix(2, 0),
					ExpectedSourceCommitId: "deadbeef",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := context.Background()
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, t := range tc.Setup {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
				}
				err = dbHandler.DBInsertApplication(ctx, transaction, "test", db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{"test"})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       gotime.Time{},
					App:           "test",
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 1, 1, "00000000-0000-0000-0000-000000000003", "deadbeef", &event.NewRelease{Environments: map[string]struct{}{"development": {}}})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, "test", []int64{1}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				var version int64 = 1
				err = dbHandler.DBWriteDeployment(ctx, transaction, db.Deployment{
					App:     "test",
					Env:     "development",
					Version: &version,
				}, 0)
				err = repo.Apply(ctx, transaction, tc.Setup...)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			sv := &VersionServiceServer{Repository: repo}
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
				TransformerMetadata: repository.TransformerMetadata{AuthorName: "author-name", AuthorEmail: "author-email"},
			},
			&repository.CreateEnvironment{
				Environment: "staging",
				Config: config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Latest: true,
					},
				},
				TransformerMetadata: repository.TransformerMetadata{AuthorName: "author-name", AuthorEmail: "author-email"},
			},
		}
	}
	fixtureRelease := func(application string, release uint64) *repository.CreateApplicationVersion {
		return &repository.CreateApplicationVersion{
			Authentication:  repository.Authentication{},
			Application:     application,
			Version:         release,
			Team:            "team-123",
			WriteCommitData: false,
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
			TransformerMetadata: repository.TransformerMetadata{AuthorName: "author-name", AuthorEmail: "author-email"},
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

	release := fixtureRelease(appName, 3)
	tcs := []*testCase{
		&testCase{
			name: "happy path",
			setup: []repository.Transformer{
				fixtureRelease(appNameOther, 1),
				fixtureRelease(appNameOther, 2),
				fixtureRelease(appName, 1),
				fixtureRelease(appName, 2),
				release,
			},
			req:  fixtureRequest(),
			want: fixtureReleaseToManifests(release),
		},
		&testCase{
			name: "request specific release",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appName, 1),
				fixtureRelease(appNameOther, 1),
				fixtureRelease(appName, 2),
				release,
				fixtureRelease(appNameOther, 2),
			),
			req:  fixtureRequest(func(req *api.GetManifestsRequest) { req.Release = "3" }),
			want: fixtureReleaseToManifests(release),
		},
		&testCase{
			name: "no release specified",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appName, 1),
				fixtureRelease(appName, 2),
				fixtureRelease(appName, 3),
			),
			req:     fixtureRequest(func(req *api.GetManifestsRequest) { req.Release = "" }),
			wantErr: status.Error(codes.InvalidArgument, "invalid release number, expected uint or 'latest'"),
		},
		&testCase{
			name: "no application specified",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appName, 1),
				fixtureRelease(appName, 2),
				fixtureRelease(appName, 3),
			),
			req:     fixtureRequest(func(req *api.GetManifestsRequest) { req.Application = "" }),
			wantErr: status.Error(codes.InvalidArgument, "no application specified"),
		},
		&testCase{
			name: "no releases for application",
			setup: append(fixtureSetupEnv(),
				fixtureRelease(appNameOther, 1),
				fixtureRelease(appNameOther, 2),
				fixtureRelease(appNameOther, 3),
			),
			req:     fixtureRequest(),
			wantErr: status.Errorf(codes.NotFound, "no releases found for application %s", appName),
		},
	}

	for _, tc := range tcs {
		tc := tc // TODO SRX-SRRONB: Remove after switching to go v1.22
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := context.Background()
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, t := range tc.setup {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
				}
				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertApplication(ctx, transaction, appNameOther, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName, appNameOther})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       gotime.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    1,
					ReleaseNumber: 2,
					Created:       gotime.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 1)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    2,
					ReleaseNumber: 3,
					Created:       gotime.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 2)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1, 2, 3}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    3,
					ReleaseNumber: 1,
					Created:       gotime.Time{},
					App:           appNameOther,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 3)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    4,
					ReleaseNumber: 2,
					Created:       gotime.Time{},
					App:           appNameOther,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 4)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    5,
					ReleaseNumber: 3,
					Created:       gotime.Time{},
					App:           appNameOther,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 5)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appNameOther, []int64{1, 2, 3}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = repo.Apply(ctx, transaction, tc.setup...)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Error in Database Operation: %v", err)
			}

			sv := &VersionServiceServer{Repository: repo}
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
