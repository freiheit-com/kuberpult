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
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/go-git/go-billy/v5"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"testing"

	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/go-git/go-billy/v5/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
		ReleaseVersionLimit: 2,
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
					Authentication:   Authentication{},
					Environment:      envAcceptance,
					Application:      appName,
					Version:          7,
					LockBehaviour:    0,
					WriteCommitData:  false,
					SourceTrain:      nil,
					Author:           "",
					TransformerEslID: 1,
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
				EslId: 0,
				App:   appName,
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
			// as of now we only have the DeployApplicationVersion and CreateEnvironmentLock transformer,
			// so we can test only this error case.
			// As soon as we have the other transformers (especially CreateEnvironment)
			// we need to add more tests here.
			Name: "create environment lock",
			Transformers: []Transformer{
				&CreateEnvironmentLock{
					Authentication:   Authentication{},
					Environment:      envAcceptance,
					LockId:           "my-lock",
					Message:          "My envAcceptance lock",
					TransformerEslID: 1,
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
					Authentication:   Authentication{},
					Environment:      envAcceptance,
					LockId:           "my-lock",
					Application:      "my-app",
					Message:          "My envAcceptance lock",
					TransformerEslID: 1,
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
					Authentication:   Authentication{},
					Environment:      envAcceptance,
					LockId:           "my-lock",
					Team:             "my-team",
					Message:          "My envAcceptance lock",
					TransformerEslID: 1,
				},
			},
			ExpectedError: errMatcher{"first apply failed, aborting: error at index 0 of transformer batch: " +
				"team 'my-team' does not exist",
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
					SourceCommitId:   "abcdef",
					SourceAuthor:     "",
					SourceMessage:    "",
					Team:             "team-123",
					DisplayVersion:   "",
					WriteCommitData:  false,
					PreviousCommit:   "",
					TransformerEslID: 1,
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
					TransformerEslID: 2,
				},
				&DeployApplicationVersion{
					Authentication:   Authentication{},
					Environment:      "staging",
					Application:      appName,
					Version:          1,
					LockBehaviour:    0,
					WriteCommitData:  true,
					SourceTrain:      nil,
					Author:           authorEmail,
					TransformerEslID: 3,
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
					Application:      appName,
					TransformerEslID: 4,
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslId, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslId), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslId:         0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslId)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslId)
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
					Team:             "my-team",
					WriteCommitData:  true,
					Version:          1,
					TransformerEslID: 1,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
				},
				&DeployApplicationVersion{
					Application:      appName,
					Environment:      "staging",
					WriteCommitData:  true,
					Version:          1,
					TransformerEslID: 2,
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
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000001/application",
					fileData: []byte(appName),
				},
				{
					path:     "commits/aa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events/00000000-0000-0000-0000-000000000001/environment",
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
			//ctx := context.Background()

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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
						err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, t.GetEslID(), "00000000-0000-0000-0000-00000000000"+strconv.Itoa(idx), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
						if err != nil {
							return err
						}
					}
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 1, "00000000-0000-0000-0000-000000000000", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslId, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslId), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslId:         0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslId)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslId)
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
		ExpectedError       error
		ExpectedFile        []*FilenameAndData
		ExpectedAuthor      *map[string]string
		ExpectedDeletedFile string
	}{
		{
			Name: "CleanupOldApplicationVersions", //ReleaseLimit is 2
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
					TransformerEslID: 1,
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
					TransformerEslID: 2,
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
					TransformerEslID: 3,
				},
				&CleanupOldApplicationVersions{
					Application: appName,
					TransformerMetadata: TransformerMetadata{
						AuthorName:  authorName,
						AuthorEmail: authorEmail,
					},
					TransformerEslID: 4,
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
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _ := setupRepositoryTestWithPath(t)
			ctx := AddGeneratorToContext(testutil.MakeTestContext(), testutil.NewIncrementalUUIDGenerator())

			dbHandler := repo.State().DBHandler
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				// setup:
				// this 'INSERT INTO' would be done one the cd-server side, so we emulate it here:
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslId, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslId), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslId:         0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslId)

				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1}, db.InitialEslId)
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

// @@@@
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
					Team:             "my-team",
					WriteCommitData:  false,
					Version:          1,
					TransformerEslID: 1,
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
					Team:             "my-team",
					WriteCommitData:  false,
					Version:          2,
					TransformerEslID: 2,
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
					TransformerEslID: 3,
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
					TransformerEslID: 4,
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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

				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 1, "00000000-0000-0000-0000-000000000001", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.NewRelease{})
				if err != nil {
					return err //bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
				}
				err = dbHandler.DBWriteNewReleaseEvent(ctx, transaction, 2, "00000000-0000-0000-0000-000000000002", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", &event.NewRelease{})
				if err != nil {
					return err //bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
				}

				err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, 3, "00000000-0000-0000-0000-000000000003", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &event.Deployment{Application: appName, Environment: "staging"})
				if err != nil {
					return err //bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
				}
				err = dbHandler.DBWriteDeploymentEvent(ctx, transaction, 4, "00000000-0000-0000-0000-000000000004", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", &event.Deployment{Application: appName, Environment: "staging"})
				if err != nil {
					return err //bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
				}

				err = dbHandler.DBWriteReplacedByEvent(ctx, transaction, 4, "00000000-0000-0000-0000-000000000005", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", &tc.Event)
				if err != nil {
					return err
				}

				err = dbHandler.DBInsertApplication(ctx, transaction, appName, db.InitialEslId, db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteAllApplications(ctx, transaction, int64(db.InitialEslId), []string{appName})
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslId:         0,
					ReleaseNumber: 1,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslId)
				err = dbHandler.DBInsertRelease(ctx, transaction, db.DBReleaseWithMetaData{
					EslId:         1,
					ReleaseNumber: 2,
					Created:       time.Time{},
					App:           appName,
					Manifests:     db.DBReleaseManifests{},
					Metadata:      db.DBReleaseMetaData{},
				}, db.InitialEslId)
				if err != nil {
					return err
				}
				err = dbHandler.DBInsertAllReleases(ctx, transaction, appName, []int64{1, 2}, db.InitialEslId)
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
