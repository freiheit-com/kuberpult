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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/go-git/go-billy/v5"

	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	envDev         = "dev"
	envAcceptance  = "acceptance"
	envAcceptance2 = "acceptance2"
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

	repoCfg := RepositoryConfig{
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
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name                string
		Transformers        []Transformer
		ExpectedError       error
		ExpectedApp         *db.DBAppWithMetaData
		ExpectedAllReleases *db.DBReleaseWithMetaData
		ExpectedFile        []*FilenameAndData
		ExpectedAuthor      *map[string]string
		ExpectedDeletedFile string
	}{
		{
			// as of now we only have the DeployApplicationVersion transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment and CreateApplicationVersion)
			// we need to add more tests here.
			Name: "generates only deployed manifest",
			Transformers: []Transformer{
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           envAcceptance,
					Application:           appName,
					Version:               7,
					LockBehaviour:         0,
					WriteCommitData:       false,
					SourceTrain:           nil,
					Author:                "",
					TransformerEslVersion: 1,
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"release of app myapp with version 7 not found",
			},
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
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedError: nil,
			ExpectedFile: []*FilenameAndData{
				{
					path:     "/applications/" + appName + "/team",
					fileData: []byte("team-123"),
				},
			},
			ExpectedApp: &db.DBAppWithMetaData{
				EslVersion: 0,
				App:        appName,
				Metadata: db.DBAppMetaData{
					Team: "team-123",
				},
			},
			ExpectedAuthor: &map[string]string{
				"Name":  authorName,
				"Email": authorEmail,
			},
		},
		{
			Name: "Should give an error when the metadata is nil",
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
			ExpectedError: errMatcher{"first apply failed, aborting: error not specific to one transformer of this batch: " +
				"transformer metadata is empty",
			},
		},
		{
			Name: "Create a single environment",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "development",
					Config:      testutil.MakeEnvConfigLatest(nil),
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "/environments/development/config.json",
					fileData: []byte(
						`{
  "upstream": {
    "latest": true
  }
}
`),
				},
			},
		},
		{
			Name: "Create a single environment twice",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigUpstream("development", nil),
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path: "/environments/staging/config.json",
					fileData: []byte(
						`{
  "upstream": {
    "environment": "development"
  }
}
`),
				},
			},
		},
		{
			Name: "Create an environment and delete it",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:        "abcdef",
					SourceAuthor:          "",
					SourceMessage:         "",
					Team:                  "team-123",
					DisplayVersion:        "",
					WriteCommitData:       false,
					PreviousCommit:        "",
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config:      testutil.MakeEnvConfigLatest(nil),
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&DeployApplicationVersion{
					Authentication:        Authentication{},
					Environment:           "staging",
					Application:           appName,
					Version:               1,
					LockBehaviour:         0,
					WriteCommitData:       true,
					SourceTrain:           nil,
					Author:                authorEmail,
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteEnvFromApp{
					Environment: "staging",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					Application:           appName,
					TransformerEslVersion: 4,
				},
			},
			ExpectedDeletedFile: "/environments/staging/applications/" + appName + "/deployed_by",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := context.Background()

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for idx, t := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
					if t.GetDBEventType() == db.EvtDeployApplicationVersion || t.GetDBEventType() == db.EvtDeployApplicationVersion {
						err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, 0, "00000000-0000-0000-0000-00000000000"+strconv.Itoa(idx+1), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
						if err != nil {
							return err
						}
					}
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if tc.ExpectedFile != nil {
				for i := range tc.ExpectedFile {
					expectedFile := tc.ExpectedFile[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("Expected no error: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(actualFileData, expectedFile.fileData) {
						t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
					}
					if tc.ExpectedAuthor != nil {
						if !cmp.Equal(updatedState.Commit.Author().Name, (*tc.ExpectedAuthor)["Name"]) {
							t.Fatalf("Expected '%v', got '%v'", (*tc.ExpectedAuthor)["Name"], updatedState.Commit.Author().Name)
						}
						if !cmp.Equal(updatedState.Commit.Author().Email, (*tc.ExpectedAuthor)["Email"]) {
							t.Fatalf("Expected '%v', got '%v'", (*tc.ExpectedAuthor)["Email"], updatedState.Commit.Author().Email)
						}
					}
				}
			}
			if tc.ExpectedDeletedFile != "" {
				updatedState := repo.State()
				fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.ExpectedDeletedFile)
				_, err := util.ReadFile(updatedState.Filesystem, fullPath)
				if err == nil {
					t.Fatalf("Expected file to be deleted but it exists: %s", fullPath)
				}
			}
		})
	}
}

func verifyContent(fs billy.Filesystem, required []*FilenameAndData) error {
	for _, contentRequirement := range required {
		if data, err := util.ReadFile(fs, contentRequirement.path); err != nil {
			return fmt.Errorf("error while opening file %s, error: %w", contentRequirement.path, err)
		} else if string(data) != string(contentRequirement.fileData) {
			return fmt.Errorf("actual file content of file '%s' is not equal to required content.\nExpected: '%s', actual: '%s'", contentRequirement.path, contentRequirement.fileData, string(data))
		}
	}
	return nil
}

func verifyMissing(fs billy.Filesystem, required []*FilenameAndData) error {
	for _, contentRequirement := range required {
		if _, err := fs.Stat(contentRequirement.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Printf("%s does not exists but is expected \n", contentRequirement.path)
				return nil
			}
			return fmt.Errorf("Error on Stat for file %s: %v\n", contentRequirement.path, err)
		}
		return fmt.Errorf("File exists %s\n", contentRequirement.path)
	}
	return nil
}

func listFiles(fs billy.Filesystem) []string {
	paths := listFilesHelper(fs, ".")
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	return paths
}

func listFilesHelper(fs billy.Filesystem, path string) []string {
	ret := make([]string, 0)

	files, err := fs.ReadDir(path)
	if err == nil {
		for _, file := range files {
			ret = append(ret, listFilesHelper(fs, fs.Join(path, file.Name()))...)
		}
	} else {
		ret = append(ret, path)
	}

	return ret
}

func TestDeploymentEvent(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name                string
		Transformers        []Transformer
		ExpectedError       error
		Event               event.Deployment
		ExpectedApp         *db.DBAppWithMetaData
		ExpectedAllReleases *db.DBReleaseWithMetaData
		ExpectedFile        []*FilenameAndData
	}{
		{
			Name: "Test Deploy Application event", //ReleaseLimit is 2
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"staging": "doesn't matter",
					},
					Team:                  "my-team",
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Application:           appName,
					Environment:           "staging",
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			Event: event.Deployment{
				Application: appName,
				Environment: "staging",
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000000/eventType",
					fileData: []byte(event.EventTypeNewRelease),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000001/environment",
					fileData: []byte("staging"),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000001/application",
					fileData: []byte(appName),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for idx, t := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
					if t.GetDBEventType() == db.EvtDeployApplicationVersion {
						err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, t.GetEslVersion(), "00000000-0000-0000-0000-00000000000"+strconv.Itoa(idx), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
						if err != nil {
							return err
						}
					}
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 1, 1, "00000000-0000-0000-0000-000000000000", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if tc.ExpectedFile != nil {
				for i := range tc.ExpectedFile {
					expectedFile := tc.ExpectedFile[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("Expected no error: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(actualFileData, expectedFile.fileData) {
						t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
					}
				}
			}
		})
	}
}

func TestReleaseTrain(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name                string
		Transformers        []Transformer
		ExpectedError       error
		Event               event.Deployment
		ExpectedApp         *db.DBAppWithMetaData
		ExpectedAllReleases *db.DBReleaseWithMetaData
		ExpectedFile        []*FilenameAndData
	}{
		{
			Name: "Trigger a deployment via a relase train with environment target",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
						},
					},
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironment{
					Environment: "staging",
					Config: config.EnvironmentConfig{
						Upstream: &config.EnvironmentConfigUpstream{
							Environment: "staging",
							Latest:      true,
						},
					},
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"production": "some production manifest 2",
						"staging":    "some staging manifest 2",
					},
					WriteCommitData:       true,
					Version:               1,
					TransformerEslVersion: 3,
					Team:                  "team-123",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Environment:           "staging",
					Application:           appName,
					Version:               1,
					WriteCommitData:       true,
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&ReleaseTrain{
					Target:                "production",
					WriteCommitData:       true,
					TransformerEslVersion: 5,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000004/application",
					fileData: []byte(appName),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000004/environment",
					fileData: []byte("production"),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000004/eventType",
					fileData: []byte("deployment"),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000004/source_train_upstream",
					fileData: []byte("staging"),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for idx, t := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
					if t.GetDBEventType() == db.EvtDeployApplicationVersion {
						err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, t.GetEslVersion(), "00000000-0000-0000-0000-00000000000"+strconv.Itoa(idx), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
						if err != nil {
							return err
						}
					}

					if t.GetDBEventType() == db.EvtReleaseTrain {
						var sourceTrainUpstream string
						sourceTrainUpstream = "staging"
						err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, t.GetEslVersion(), "00000000-0000-0000-0000-00000000000"+strconv.Itoa(idx), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "production", SourceTrainUpstream: &sourceTrainUpstream})
						if err != nil {
							return err
						}
					}
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 3, 1, "00000000-0000-0000-0000-000000000000", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslVersion)
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteEnvironment(ctx, transaction, "staging", config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: "staging",
						Latest:      true,
					},
				}, []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteEnvironment(ctx, transaction, "production", config.EnvironmentConfig{
					Upstream: &config.EnvironmentConfigUpstream{
						Environment: "staging",
					},
				}, []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllEnvironments(ctx, transaction, []string{"staging", "production"})
				if err != nil {
					return err
				}
				var v int64
				v = 1
				err = dbHandler.DBWriteDeployment(ctx, transaction, db.Deployment{
					App:           appName,
					Env:           "production",
					Version:       &v,
					TransformerID: 5,
				}, 10, false)
				if err != nil {
					return err
				}

				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if tc.ExpectedFile != nil {
				for i := range tc.ExpectedFile {
					expectedFile := tc.ExpectedFile[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("Expected no error: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(actualFileData, expectedFile.fileData) {
						t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
					}
				}
			}
		})
	}
}

func TestCleanupOldApplicationVersions(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name                string
		Transformers        []Transformer
		MinorRelease        uint64
		PrepublishRelease   uint64
		ExpectedError       error
		ExpectedFile        []*FilenameAndData
		ExpectedAuthor      *map[string]string
		ExpectedDeletedFile string
	}{
		{
			Name: "CleanupOldApplicationVersion without deleting any", //ReleaseLimit is 2
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        2,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        3,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CleanupOldApplicationVersions{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "/applications/" + appName + "/releases/3/source_commit_id",
					fileData: []byte("123456789abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/2/source_commit_id",
					fileData: []byte("abcdef"),
				},
			},
			ExpectedAuthor: &map[string]string{"Name": authorName, "Email": authorEmail},
		},
		{
			Name: "CleanupOldApplicationVersions deleting one", //ReleaseLimit is 2
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        2,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        3,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        4,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
				&CleanupOldApplicationVersions{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 5,
				},
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "/applications/" + appName + "/releases/3/source_commit_id",
					fileData: []byte("123456789abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/2/source_commit_id",
					fileData: []byte("abcdef"),
				},
			},
			ExpectedAuthor:      &map[string]string{"Name": authorName, "Email": authorEmail},
			ExpectedDeletedFile: "/applications/" + appName + "/releases/1/source_commit_id",
		},
		{
			Name: "CleanupOldApplicationVersions with a minor release", //ReleaseLimit is 2
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        2,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        3,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        4,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
				&CleanupOldApplicationVersions{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 5,
				},
			},
			MinorRelease: 3,
			ExpectedFile: []*FilenameAndData{
				{
					path:     "/applications/" + appName + "/releases/3/source_commit_id",
					fileData: []byte("123456789abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/2/source_commit_id",
					fileData: []byte("abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/1/source_commit_id",
					fileData: []byte("123456789"),
				},
			},
			ExpectedAuthor: &map[string]string{"Name": authorName, "Email": authorEmail},
		},
		{
			Name: "CleanupOldApplicationVersions with a prepublish release", //ReleaseLimit is 2
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        2,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        3,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        4,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
						envDev:        "mani-1-dev",
					},
					SourceCommitId:  "123456789abcdef",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
				&CleanupOldApplicationVersions{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 5,
				},
			},
			PrepublishRelease: 3,
			ExpectedFile: []*FilenameAndData{
				{
					path:     "/applications/" + appName + "/releases/3/source_commit_id",
					fileData: []byte("123456789abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/2/source_commit_id",
					fileData: []byte("abcdef"),
				},
				{
					path:     "/applications/" + appName + "/releases/1/source_commit_id",
					fileData: []byte("123456789"),
				},
			},
			ExpectedAuthor: &map[string]string{"Name": authorName, "Email": authorEmail},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)

				if tc.MinorRelease != 0 {
					err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
						EslVersion:    1,
						ReleaseNumber: tc.MinorRelease,
						Created:       time.Time{},
						App:           appName,
						Manifests:     db.DBReleaseManifests{},
						Metadata: db.DBReleaseMetaData{
							IsMinor: true,
						},
					}, 1)
				}
				if tc.PrepublishRelease != 0 {
					err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
						EslVersion:    1,
						ReleaseNumber: tc.PrepublishRelease,
						Created:       time.Time{},
						App:           appName,
						Manifests:     db.DBReleaseManifests{},
						Metadata: db.DBReleaseMetaData{
							IsPrepublish: true,
						},
					}, 1)
				}

				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}

			if tc.ExpectedFile != nil {
				for i := range tc.ExpectedFile {
					expectedFile := tc.ExpectedFile[i]
					updatedState := repo.State()
					fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), expectedFile.path)
					actualFileData, err := util.ReadFile(updatedState.Filesystem, fullPath)
					if err != nil {
						t.Fatalf("Expected no error: %v path=%s", err, fullPath)
					}

					if !cmp.Equal(actualFileData, expectedFile.fileData) {
						t.Fatalf("Expected '%v', got '%v'", string(expectedFile.fileData), string(actualFileData))
					}
					if tc.ExpectedAuthor != nil {
						if !cmp.Equal(updatedState.Commit.Author().Name, (*tc.ExpectedAuthor)["Name"]) {
							t.Fatalf("Expected '%v', got '%v'", (*tc.ExpectedAuthor)["Name"], updatedState.Commit.Author().Name)
						}
						if !cmp.Equal(updatedState.Commit.Author().Email, (*tc.ExpectedAuthor)["Email"]) {
							t.Fatalf("Expected '%v', got '%v'", (*tc.ExpectedAuthor)["Email"], updatedState.Commit.Author().Email)
						}
					}
				}
			}
			if tc.ExpectedDeletedFile != "" {
				updatedState := repo.State()
				fullPath := updatedState.Filesystem.Join(updatedState.Filesystem.Root(), tc.ExpectedDeletedFile)
				_, err := util.ReadFile(updatedState.Filesystem, fullPath)
				if err == nil {
					t.Fatalf("Expected file to be deleted but it exists: %s", fullPath)
				}
			}
		})
	}
}

func TestReplacedByEvents(t *testing.T) {
	const appName = "myapp"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name                string
		Transformers        []Transformer
		ExpectedError       error
		Event               event.ReplacedBy
		ExpectedApp         *db.DBAppWithMetaData
		ExpectedAllReleases *db.DBReleaseWithMetaData
		ExpectedFile        []*FilenameAndData
	}{
		{
			Name: "Test Replaced By event",
			Transformers: []Transformer{
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Manifests: map[string]string{
						"staging": "doesn't matter",
					},
					Team:                  "my-team",
					WriteCommitData:       false,
					Version:               1,
					TransformerEslVersion: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateApplicationVersion{
					Application:    appName,
					SourceCommitId: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					Manifests: map[string]string{
						"staging": "doesn't matter",
					},
					Team:                  "my-team",
					WriteCommitData:       false,
					Version:               2,
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					PreviousCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				},
				&DeployApplicationVersion{
					Authentication:  Authentication{},
					Environment:     "staging",
					Application:     appName,
					Version:         1,
					LockBehaviour:   1,
					WriteCommitData: true,
					SourceTrain:     nil,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&DeployApplicationVersion{
					Authentication:  Authentication{},
					Environment:     "staging",
					Application:     appName,
					Version:         2,
					LockBehaviour:   1,
					WriteCommitData: true,
					SourceTrain:     nil,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
			},
			Event: event.ReplacedBy{
				Application:       appName,
				Environment:       "staging",
				CommitIDtoReplace: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			ExpectedFile: []*FilenameAndData{
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000005/eventType",
					fileData: []byte(event.EventTypeReplaceBy),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000005/application",
					fileData: []byte(appName),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000005/commit",
					fileData: []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000005/environment",
					fileData: []byte("staging"),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, t := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
				}

				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 1, 1, "00000000-0000-0000-0000-000000000001", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 2, 1, "00000000-0000-0000-0000-000000000002", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", &event.NewRelease{})
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, 3, "00000000-0000-0000-0000-000000000003", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, 4, "00000000-0000-0000-0000-000000000004", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", &event.Deployment{Application: appName, Environment: "staging"})
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteReplacedByEvent(ctx, transaction, 4, "00000000-0000-0000-0000-000000000005", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &tc.Event)
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    1,
					ReleaseNumber: 2,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1, 2}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				// actual transformer to be tested:
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.ExpectedFile); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestCreateUndeployApplicationVersion(t *testing.T) {
	const appName = "app1"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name          string
		Transformers  []Transformer
		expectedError error
		expectedData  []*FilenameAndData
	}{
		{
			Name: "successfully create undeploy version - should work",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: "acceptance",
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Application: appName,
					Manifests: map[string]string{
						envAcceptance: "acceptance",
					},
					WriteCommitData: true,
					Version:         1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					SourceCommitId:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					TransformerEslVersion: 2,
					Team:                  "team-123",
				},
				&CreateUndeployApplicationVersion{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
			},
			expectedData: []*FilenameAndData{
				{
					path:     "applications/app1/releases/2/environments/acceptance/manifests.yaml",
					fileData: []byte(" "),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, t := range tc.Transformers {
					err := dbHandler.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: t.GetMetadata().AuthorName, AuthorEmail: t.GetMetadata().AuthorEmail})
					if err != nil {
						return err
					}
				}

				envConfig := config.EnvironmentConfig{
					Upstream:         nil,
					ArgoCd:           nil,
					EnvironmentGroup: nil,
				}
				err = dbHandler.DBWriteEnvironment(ctx, transaction, envAcceptance, envConfig, []string{})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllEnvironments(ctx, transaction, []string{envAcceptance})
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 2, 1, "00000000-0000-0000-0000-000000000001", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslVersion, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslVersion), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, 0)
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslVersion:    0,
					ReleaseNumber: 2,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1, 2}, db.InitialEslVersion)
				if err != nil {
					return err
				}
				err = repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			updatedState := repo.State()
			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestLocks(t *testing.T) {
	const appName = "app1"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedData    []*FilenameAndData
		expectedMissing []*FilenameAndData
		expectedMessage string
		expectedError   error
	}{
		{
			Name: "Create environment lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedData: []*FilenameAndData{
				{
					path:     "/environments/acceptance/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Create environment lock - env does not exist",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironmentLock{
					Environment:           "non-existent-env",
					LockId:                "l123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: errMatcher{msg: "first apply failed, aborting: error at index 0 of transformer batch: could not access environment information on: 'environments/non-existent-env': file does not exist"},
			expectedMissing: []*FilenameAndData{
				{
					path:     "/environments/acceptance/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Delete environment lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteEnvironmentLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedMissing: []*FilenameAndData{
				{
					path: "/environments/acceptance/locks/l123/created_by_email",
				},
				{
					path: "/environments/acceptance/locks/l123/created_by_name",
				},
				{
					path: "/environments/acceptance/locks/l123/message",
				},
			},
		},
		{
			Name: "Create App lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Application:           appName,
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedData: []*FilenameAndData{
				{

					path:     "/environments/acceptance/applications/app1/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/applications/app1/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/applications/app1/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Delete App lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentApplicationLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Application:           appName,
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteEnvironmentApplicationLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Application:           appName,
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedMissing: []*FilenameAndData{
				{

					path:     "/environments/acceptance/applications/app1/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/applications/app1/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/applications/app1/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Create Team lock",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentTeamLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Team:                  "team-123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedData: []*FilenameAndData{
				{

					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Delete Team lock - team does not exist",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentTeamLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Team:                  "team-123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteEnvironmentTeamLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Team:                  "team-123",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: errMatcher{
				msg: "first apply failed, aborting: error at index 0 of transformer batch: rpc error: code = InvalidArgument desc = cannot delete environment team lock: invalid team: 'team-'",
			},
			expectedMissing: []*FilenameAndData{
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
		{
			Name: "Delete Team lock - team does not exist",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentTeamLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Team:                  "team-123",
					Message:               "none",
					TransformerEslVersion: 2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeleteEnvironmentTeamLock{
					Environment:           envAcceptance,
					LockId:                "l123",
					Team:                  "team-",
					TransformerEslVersion: 3,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: errMatcher{
				msg: "first apply failed, aborting: error at index 0 of transformer batch: rpc error: code = InvalidArgument desc = cannot delete environment team lock: invalid team: 'team-'",
			},
			expectedData: []*FilenameAndData{
				{

					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_email",
					fileData: []byte(authorEmail),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/created_by_name",
					fileData: []byte(authorName),
				},
				{
					path:     "/environments/acceptance/teams/team-123/locks/l123/message",
					fileData: []byte("none"),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err2 := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err2 != nil {
					t.Fatal(err2)

				}
				//populate the database
				for _, tr := range tc.Transformers {
					err2 := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err2 != nil {
						t.Fatal(err2)
					}
					if tr.GetDBEventType() == db.EvtCreateEnvironmentLock {
						concreteTransformer := tr.(*CreateEnvironmentLock)
						err2 = dbHandler.DBWriteEnvironmentLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtDeleteEnvironmentLock {
						concreteTransformer := tr.(*DeleteEnvironmentLock)
						err2 = dbHandler.DBDeleteEnvironmentLock(ctx, transaction, concreteTransformer.Environment, concreteTransformer.LockId)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtCreateApplicationVersion {
						concreteTransformer := tr.(*CreateApplicationVersion)
						err2 = dbHandler.DBInsertApplication(ctx, transaction, concreteTransformer.Application, 0, db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBWriteAllApplications(ctx, transaction, 0, []string{appName})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{int64(concreteTransformer.Version)}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
							EslVersion:    0,
							ReleaseNumber: concreteTransformer.Version,
							App:           concreteTransformer.Application,
							Manifests: db.DBReleaseManifests{
								Manifests: concreteTransformer.Manifests,
							},
							Deleted: false,
						}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtCreateEnvironmentApplicationLock {
						concreteTransformer := tr.(*CreateEnvironmentApplicationLock)
						err2 = dbHandler.DBWriteApplicationLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, concreteTransformer.Application, db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtDeleteEnvironmentApplicationLock {
						concreteTransformer := tr.(*DeleteEnvironmentApplicationLock)
						err2 = dbHandler.DBDeleteApplicationLock(ctx, transaction, concreteTransformer.Environment, concreteTransformer.Application, concreteTransformer.LockId)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtCreateEnvironmentTeamLock {
						concreteTransformer := tr.(*CreateEnvironmentTeamLock)

						err2 = dbHandler.DBWriteTeamLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, concreteTransformer.Team, db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtDeleteEnvironmentTeamLock {
						concreteTransformer := tr.(*DeleteEnvironmentTeamLock)
						err2 = dbHandler.DBDeleteTeamLock(ctx, transaction, concreteTransformer.Environment, concreteTransformer.Team, concreteTransformer.LockId)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
				}
				var commitMsg []string
				err := repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedMessage, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})

			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
			}
			updatedState := repo.State()

			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.expectedMissing); err != nil {
				t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestCreateUndeployLogic(t *testing.T) {
	const appName = "app1"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedData    []*FilenameAndData
		expectedMissing []*FilenameAndData
	}{
		{

			Name: "Create undeploy application version and deploy it",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance: "mani-1-acc",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
			},
			expectedData: []*FilenameAndData{
				{
					path:     "/applications/app1/releases/2/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "/environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
			},
		},
		{
			Name: "Try Create undeploy application version, but it is locked",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance2, Latest: true}},
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				//We need to deploy manually due to discrepancy between DB and manifest on testing env, causing locks to not work as the previous version isn't the lastest
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance,
					Version:         1,
					LockBehaviour:   2,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance2,
					Version:         1,
					LockBehaviour:   api.LockBehavior_RECORD,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",

					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance2,
					LockId:                "my-lock",
					Message:               "Acceptance 2 is locked",
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},

					TransformerEslVersion: 5,
				},
			},
			expectedData: []*FilenameAndData{
				{ //There is an undeploy version
					path:     "/applications/app1/releases/2/undeploy",
					fileData: []byte(""),
				},
				{ //The first env has the undeploy version deployed
					path:     "environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{ //The second env has the undeploy version *queued*
					path:     "environments/acceptance2/applications/app1/queued_version/undeploy",
					fileData: []byte(""),
				},
			},
			expectedMissing: []*FilenameAndData{
				{ //The second env does NOT have the undeploy version
					path:     "environments/acceptance2/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err2 := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err2 != nil {
					t.Fatal(err2)

				}
				//populate the database
				for _, tr := range tc.Transformers {
					err2 := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err2 != nil {
						t.Fatal(err2)
					}
					if tr.GetDBEventType() == db.EvtCreateApplicationVersion {
						concreteTransformer := tr.(*CreateApplicationVersion)
						err2 = dbHandler.DBInsertApplication(ctx, transaction, concreteTransformer.Application, 0, db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBWriteAllApplications(ctx, transaction, 0, []string{appName})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{int64(concreteTransformer.Version)}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
							EslVersion:    0,
							ReleaseNumber: concreteTransformer.Version,
							App:           concreteTransformer.Application,
							Manifests: db.DBReleaseManifests{
								Manifests: concreteTransformer.Manifests,
							},
							Deleted: false,
						}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
					}

					if tr.GetDBEventType() == db.EvtCreateEnvironmentLock {
						concreteTransformer := tr.(*CreateEnvironmentLock)
						err2 = dbHandler.DBWriteEnvironmentLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBWriteAllEnvironmentLocks(ctx, transaction, 0, concreteTransformer.Environment, []string{concreteTransformer.LockId})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtCreateUndeployApplicationVersion {
						err2 = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
							EslVersion:    1,
							ReleaseNumber: 2,
							App:           appName,
							Manifests: db.DBReleaseManifests{
								Manifests: map[string]string{ //empty manifest
									"": "",
								},
							},
							Metadata: db.DBReleaseMetaData{
								SourceAuthor:    "",
								SourceCommitId:  "",
								SourceMessage:   "",
								DisplayVersion:  "",
								UndeployVersion: true,
							},
							Created: time.Now(),
							Deleted: false,
						}, 1)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases, err2 := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases.Metadata.Releases = append(allReleases.Metadata.Releases, int64(2))
						err2 = dbHandler.DBInsertAllReleases(ctx, transaction, appName, allReleases.Metadata.Releases, 1)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases, err2 = dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, appName)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
				}
				err := repo.Apply(ctx, transaction, tc.Transformers...)
				if err != nil {
					return err
				}

				return nil
			})

			if err != nil {
				t.Fatalf("error not expected but got: \n%v", err)

			}
			updatedState := repo.State()

			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.expectedMissing); err != nil {
				t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}

func TestUndeployLogic(t *testing.T) {
	const appName = "app1"
	const authorName = "testAuthorName"
	const authorEmail = "testAuthorEmail@example.com"
	environmentConfigAcceptance := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance, Latest: true}}
	environmentConfigAcceptance2 := config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Environment: envAcceptance2, Latest: true}}
	tcs := []struct {
		Name            string
		Transformers    []Transformer
		expectedData    []*FilenameAndData
		expectedMissing []*FilenameAndData
		expectedMessage string
		expectedError   error
	}{
		{
			Name: "Undeploy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				//We need to deploy manually due to discrepancy between DB and manifest on testing env, causing locks to not work as the previous version isn't the lastest
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance,
					Version:         1,
					LockBehaviour:   2,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 4,
				},
				&DeployApplicationVersion{
					Application:     appName,
					Environment:     envAcceptance2,
					Version:         1,
					LockBehaviour:   api.LockBehavior_RECORD,
					WriteCommitData: false,
					SourceTrain:     nil,
					Author:          "",

					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 5,
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},

					TransformerEslVersion: 6,
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 7,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedMissing: []*FilenameAndData{
				{
					path:     "environments/acceptance2/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "/applications/app1/releases/2/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{
					path:     "environments/acceptance2/applications/app1/queued_version/undeploy",
					fileData: []byte(""),
				},
			},
		},
		{
			Name: "Try to undeploy application, with no undeploy versions",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance2,
					LockId:                "my-lock",
					Message:               "Acceptance 2 is locked",
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 6,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: errMatcher{msg: "error within transaction: first apply failed, aborting: error at index 0 of transformer batch: UndeployApplication: error last release is not un-deployed application version of 'app1'"},
		},
		{
			Name: "Try to undeploy application, but no all envs with undeploy",
			Transformers: []Transformer{
				&CreateEnvironment{
					Environment: envAcceptance,
					Config:      environmentConfigAcceptance,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 1,
				},
				&CreateEnvironment{
					Environment: envAcceptance2,
					Config:      environmentConfigAcceptance2,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 2,
				},
				&CreateApplicationVersion{
					Authentication: Authentication{},
					Version:        1,
					Application:    appName,
					Manifests: map[string]string{
						envAcceptance:  "mani-1-acc",
						envAcceptance2: "e2",
					},
					SourceCommitId:  "",
					SourceAuthor:    "",
					SourceMessage:   "",
					Team:            "team-123",
					DisplayVersion:  "",
					WriteCommitData: false,
					PreviousCommit:  "",
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslVersion: 3,
				},
				&CreateEnvironmentLock{
					Environment:           envAcceptance2,
					LockId:                "my-lock",
					Message:               "Acceptance 2 is locked",
					TransformerEslVersion: 4,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&CreateUndeployApplicationVersion{
					Authentication:  Authentication{},
					Application:     appName,
					WriteCommitData: false,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},

					TransformerEslVersion: 5,
				},
				&UndeployApplication{
					Application:           appName,
					TransformerEslVersion: 6,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
			},
			expectedError: errMatcher{msg: "error within transaction: first apply failed, aborting: error at index 0 of transformer batch: UndeployApplication(repo):" +
				" error cannot un-deploy application '" + appName + "' the release on 'acceptance2' is not un-deployed: 'environments/acceptance2/applications/" + appName + "/version/undeploy'"},
			expectedData: []*FilenameAndData{},
			expectedMissing: []*FilenameAndData{
				{ //The first env has the undeploy version deployed
					path:     "environments/acceptance/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{ //The second env does NOT have the undeploy version
					path:     "environments/acceptance2/applications/app1/version/undeploy",
					fileData: []byte(""),
				},
				{ //There is no undeploy version, because all releases have been deleted
					path:     "/applications/app1/releases/2/undeploy",
					fileData: []byte(""),
				},
				{ //The second env has the undeploy version *queued*
					path:     "environments/acceptance2/applications/app1/queued_version",
					fileData: []byte(""),
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			//t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransactionR(ctx, 0, false, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err2 := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err2 != nil {
					t.Fatal(err2)
				}
				err2 = dbHandler.DBWriteEnvironment(ctx, transaction, envAcceptance, environmentConfigAcceptance, []string{appName})
				if err2 != nil {
					return err2
				}
				err2 = dbHandler.DBWriteEnvironment(ctx, transaction, envAcceptance2, environmentConfigAcceptance2, []string{appName})
				if err2 != nil {
					return err2
				}
				err2 = dbHandler.DBWriteAllEnvironments(ctx, transaction, []string{envAcceptance, envAcceptance2})
				if err2 != nil {
					return err2
				}
				//populate the database
				for _, tr := range tc.Transformers {
					err2 := dbHandler.DBWriteEslEventInternal(ctx, tr.GetDBEventType(), transaction, t, db.ESLMetadata{AuthorName: tr.GetMetadata().AuthorName, AuthorEmail: tr.GetMetadata().AuthorEmail})
					if err2 != nil {
						t.Fatal(err2)
					}
					if tr.GetDBEventType() == db.EvtCreateApplicationVersion {
						concreteTransformer := tr.(*CreateApplicationVersion)
						err2 = dbHandler.DBInsertApplication(ctx, transaction, concreteTransformer.Application, 0, db.AppStateChangeCreate, db.DBAppMetaData{Team: concreteTransformer.Team})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBWriteAllApplications(ctx, transaction, 0, []string{concreteTransformer.Application})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertAllReleases(ctx, transaction, concreteTransformer.Application, []int64{int64(concreteTransformer.Version)}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
							EslVersion:    0,
							ReleaseNumber: concreteTransformer.Version,
							App:           concreteTransformer.Application,
							Manifests: db.DBReleaseManifests{
								Manifests: concreteTransformer.Manifests,
							},
							Deleted: false,
						}, 0)
						if err2 != nil {
							t.Fatal(err2)
						}
					}

					if tr.GetDBEventType() == db.EvtCreateEnvironmentLock {
						concreteTransformer := tr.(*CreateEnvironmentLock)
						err2 = dbHandler.DBWriteEnvironmentLock(ctx, transaction, concreteTransformer.LockId, concreteTransformer.Environment, db.LockMetadata{
							CreatedByName:  concreteTransformer.AuthorName,
							CreatedByEmail: concreteTransformer.AuthorEmail,
							Message:        concreteTransformer.Message,
							CiLink:         "", //not transported to repo
						})
						if err2 != nil {
							t.Fatal(err2)
						}
						err2 = dbHandler.DBWriteAllEnvironmentLocks(ctx, transaction, 0, concreteTransformer.Environment, []string{concreteTransformer.LockId})
						if err2 != nil {
							t.Fatal(err2)
						}
					}
					if tr.GetDBEventType() == db.EvtCreateUndeployApplicationVersion {
						concreteTransformer := tr.(*CreateUndeployApplicationVersion)
						err2 = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
							EslVersion:    1,
							ReleaseNumber: 2,
							App:           concreteTransformer.Application,
							Manifests: db.DBReleaseManifests{
								Manifests: map[string]string{ //empty manifest
									"": "",
								},
							},
							Metadata: db.DBReleaseMetaData{
								SourceAuthor:    "",
								SourceCommitId:  "",
								SourceMessage:   "",
								DisplayVersion:  "",
								UndeployVersion: true,
							},
							Created: time.Now(),
							Deleted: false,
						}, 1)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases, err2 := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, concreteTransformer.Application)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases.Metadata.Releases = append(allReleases.Metadata.Releases, int64(2))
						err2 = dbHandler.DBInsertAllReleases(ctx, transaction, concreteTransformer.Application, allReleases.Metadata.Releases, 1)
						if err2 != nil {
							t.Fatal(err2)
						}
						allReleases, err2 = dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, concreteTransformer.Application)
						if err2 != nil {
							t.Fatal(err2)
						}
					}
				}
				var commitMsg []string
				for _, t := range tc.Transformers {
					err := repo.Apply(ctx, transaction, t)
					if err != nil {
						return err
					}
					err = repo.PushRepo(ctx) // just for testing!
					if err != nil {
						return err
					}
				}
				actualMsg := ""
				// note that we only check the LAST error here:
				if len(commitMsg) > 0 {
					actualMsg = commitMsg[len(commitMsg)-1]
				}
				if diff := cmp.Diff(tc.expectedMessage, actualMsg); diff != "" {
					t.Errorf("commit message mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})

			if err != nil {
				if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
			}
			updatedState := repo.State()

			if err := verifyContent(updatedState.Filesystem, tc.expectedData); err != nil {
				t.Fatalf("Error while verifying content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
			if err := verifyMissing(updatedState.Filesystem, tc.expectedMissing); err != nil {
				t.Fatalf("Error while verifying missing content: %v.\nFilesystem content:\n%s", err, strings.Join(listFiles(updatedState.Filesystem), "\n"))
			}
		})
	}
}
