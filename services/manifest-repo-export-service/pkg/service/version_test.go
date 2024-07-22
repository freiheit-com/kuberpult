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
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"path"
	"path/filepath"
	"os/exec"
	"github.com/freiheit-com/kuberpult/pkg/time"
	"testing"
	gotime "time"

	"github.com/freiheit-com/kuberpult/pkg/db"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

func setupRepositoryTestWithDB(t *testing.T, dbConfig *db.DBConfig) (repository.Repository, error) {
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	cmd.Start()
	cmd.Wait()
	t.Logf("test created dir: %s", localDir)

	repoCfg := repository.RepositoryConfig{
		URL:                    remoteDir,
		Path:                   localDir,
		CommitterEmail:         "kuberpult@freiheit.com",
		CommitterName:          "kuberpult",
		EnvironmentConfigsPath: filepath.Join(remoteDir, "..", "environment_configs.json"),
		ArgoCdGenerateFiles:    true,
	}
	if dbConfig != nil {
		dbConfig.DbHost = dir

		migErr := db.RunDBMigrations(*dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		db, err := db.Connect(*dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = db
	}

	repo, err := repository.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, nil
}

func setupRepositoryTest(t *testing.T) (repository.Repository, error) {
	return setupRepositoryTestWithDB(t, nil)
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
					ExpectedDeployedAt: gotime.Unix(2, 0),
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
					ExpectedDeployedAt:     gotime.Unix(2, 0),
					ExpectedSourceCommitId: "deadbeef",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			repo, err := setupRepositoryTest(t)
			ctx := testutil.MakeTestContext()
			if err != nil {
				t.Fatalf("error setting up repository test: %v", err)
			}
			sv := &VersionServiceServer{Repository: repo}
			err = repo.State().DBHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
				for i, transformer := range tc.Setup {
					now := gotime.Unix(int64(i), 0)
					ctx := time.WithTimeNow(testutil.MakeTestContext(), now)
					err := repo.Apply(ctx, tx, transformer)
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
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
