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

package repository

import (
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"os/exec"
	"path"
	"testing"
)

const (
	envAcceptance = "acceptance"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func setupRepositoryTestWithPath(t *testing.T) (Repository, string) {
	migrationsPath, err := testutil.CreateMigrationsPath()
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

	repoCfg := RepositoryConfig{
		URL:                 remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
	}

	if dbConfig != nil {
		dbConfig.DbHost = dir

		migErr := db.RunDBMigrations(*dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		dbHandler, err := db.Connect(*dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = dbHandler
	}

	repo, err := New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, remoteDir
}

func TestTransformerWorksWithDb(t *testing.T) {
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		ExpectedError error
	}{
		{
			// as of now we only have the DeployApplicationVersion transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment and CreateApplicationVersion)
			// we need to add more tests here.
			Name: "generates only deployed manifest",
			Transformers: []Transformer{
				&DeployApplicationVersion{
					Authentication:  Authentication{},
					Environment:     envAcceptance,
					Application:     "myapp",
					Version:         7,
					LockBehaviour:   0,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"deployment failed: could not open manifest for app myapp with release 7 on env acceptance " +
				"'applications/myapp/releases/7/environments/acceptance/manifests.yaml': " +
				"file does not exist"},
		},
		{
			// as of now we only have the DeployApplicationVersion and CreateEnvironmentLock transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment)
			// we need to add more tests here.
			Name: "create environment lock",
			Transformers: []Transformer{
				&CreateEnvironmentLock{
					Authentication: Authentication{},
					Environment:    envAcceptance,
					LockId:         "my-lock",
					Message:        "My envAcceptance lock",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"error accessing dir \"environments/acceptance\": file does not exist",
			},
		},
		{
			// as of now we only have the DeployApplicationVersion and Create/DeleteEnvironmentLock transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment)
			// we need to add more tests here.
			Name: "delete environment lock",
			Transformers: []Transformer{
				&DeleteEnvironmentLock{
					Authentication: Authentication{},
					Environment:    envAcceptance,
					LockId:         "my-lock",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"rpc error: code = FailedPrecondition desc = error: directory environments/acceptance/locks/my-lock for env lock does not exist",
			},
		},
		{
			// as of now we only have the DeployApplicationVersion and CreateEnvironmentLock transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment)
			// we need to add more tests here.
			Name: "create applications lock",
			Transformers: []Transformer{
				&CreateEnvironmentApplicationLock{
					Authentication: Authentication{},
					Environment:    envAcceptance,
					LockId:         "my-lock",
					Application:    "my-app",
					Message:        "My envAcceptance lock",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"error accessing dir \"environments/acceptance\": file does not exist",
			},
		},
		{
			// as of now we only have the DeployApplicationVersion and Create/DeleteApplicationLock transformer,
			// so we can test only this error case.
			Name: "delete application lock",
			Transformers: []Transformer{
				&DeleteEnvironmentApplicationLock{
					Authentication: Authentication{},
					Environment:    envAcceptance,
					LockId:         "my-lock",
					Application:    "my-app",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"rpc error: code = FailedPrecondition desc = error: directory environments/acceptance/applications/my-app/locks/my-lock for app lock does not exist",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := context.Background()
			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				err := repo.Apply(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				row, err := dbHandler.DBReadEslEventLaterThan(ctx, transaction, -1)
				if err != nil {
					return err
				}
				if row != nil {
					t.Errorf("expected eslEvent table to be empty, but got:\n%v", row)
				}
				deployment, err := dbHandler.DBSelectAnyDeployment(ctx, transaction)
				if err != nil {
					return err
				}
				if deployment != nil {
					t.Errorf("expected deployment table to be empty, but got:\n%v", deployment)
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
