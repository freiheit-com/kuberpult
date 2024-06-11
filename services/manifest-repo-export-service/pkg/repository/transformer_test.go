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
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"os/exec"
	"path"
	"testing"
)

const (
	envDev        = "dev"
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

type FilenameAndData struct {
	path     string
	fileData []byte
}

func TestTransformerWorksWithDb(t *testing.T) {
	const appName = "myapp"
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		ExpectedError error
		ExpectedApp   *db.DBAppWithMetaData
		ExpectedFile  *FilenameAndData
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
					Application:     appName,
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
			ExpectedApp:  nil,
			ExpectedFile: nil,
		},
		{
			// as of now we only have the DeployApplicationVersion transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment and CreateApplicationVersion)
			// we need to add more tests here.
			Name: "creates a new release",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        7,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
				},
			},
			ExpectedError: nil,
			ExpectedFile: &FilenameAndData{
				path:     "/applications/" + appName + "/team",
				fileData: []byte("team-123"),
			},
			ExpectedApp: &db.DBAppWithMetaData{
				EslId: 0,
				App:   appName,
				Metadata: db.DBAppMetaData{
					Team: "team-123",
				},
			},
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
			Name: "create team lock",
			Transformers: []Transformer{
				&CreateEnvironmentTeamLock{
					Authentication: Authentication{},
					Environment:    envAcceptance,
					LockId:         "my-lock",
					Team:           "my-team",
					Message:        "My envAcceptance lock",
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"team 'my-team' does not exist",
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
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslId, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				// actual transformer to be tested:
				err = repo.Apply(testutil.MakeTestContext(), transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}

			if tc.ExpectedFile != nil {
				updatedState := repo.State()
				fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.ExpectedFile.path)
				actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
				if err != nil {
					t.Fatalf("Expected no error: %v path=%s", err, fullPath)
				}

				if !cmp.Equal(actualFileData, tc.ExpectedFile.fileData) {
					t.Fatalf("Expected '%v', got '%v'", string(tc.ExpectedFile.fileData), string(actualFileData))
				}
			}
		})
	}
}
