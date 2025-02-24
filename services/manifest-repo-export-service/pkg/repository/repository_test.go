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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	git "github.com/libgit2/git2go/v34"
)

func TestRetrySsh(t *testing.T) {
	tcs := []struct {
		Name              string
		NumOfFailures     int
		ExpectedNumOfCall int
		ExpectedResponse  error
		CustomResponse    error
	}{
		{
			Name:              "No retries success from 1st try",
			NumOfFailures:     0,
			ExpectedNumOfCall: 1,
			ExpectedResponse:  nil,
			CustomResponse:    nil,
		}, {
			Name:              "Success after the 4th attempt",
			NumOfFailures:     4,
			ExpectedNumOfCall: 5,
			ExpectedResponse:  nil,
			CustomResponse:    &git.GitError{Message: "mock error"},
		}, {
			Name:              "Fail after the 6th attempt",
			NumOfFailures:     6,
			ExpectedNumOfCall: 6,
			ExpectedResponse:  &git.GitError{Message: "max number of retries exceeded error"},
			CustomResponse:    &git.GitError{Message: "max number of retries exceeded error"},
		}, {
			Name:              "Do not retry after a permanent error",
			NumOfFailures:     1,
			ExpectedNumOfCall: 1,
			ExpectedResponse:  &git.GitError{Message: "permanent error"},
			CustomResponse:    &git.GitError{Message: "permanent error", Code: git.ErrorCodeNonFastForward},
		}, {
			Name:              "Fail after the 6th attempt = Max number of retries ",
			NumOfFailures:     12,
			ExpectedNumOfCall: 6,
			ExpectedResponse:  &git.GitError{Message: "max number of retries exceeded error"},
			CustomResponse:    nil,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo := &repository{}
			counter := 0
			repo.backOffProvider = func() backoff.BackOff {
				return backoff.WithMaxRetries(&backoff.ZeroBackOff{}, 5)
			}
			resp := repo.Push(testutil.MakeTestContext(), func() error {
				counter++
				if counter > tc.NumOfFailures {
					return nil
				}
				if counter == tc.NumOfFailures { //  Custom response
					return tc.CustomResponse
				}
				if counter == 6 { // max number of retries
					return &git.GitError{Message: "max number of retries exceeded error"}
				}
				return &git.GitError{Message: fmt.Sprintf("mock error %d", counter)}
			})

			if resp == nil || tc.ExpectedResponse == nil {
				if resp != tc.ExpectedResponse {
					t.Fatalf("new: expected '%v',  got '%v'", tc.ExpectedResponse, resp)
				}
			} else if resp.Error() != tc.ExpectedResponse.Error() {
				t.Fatalf("new: expected '%v',  got '%v'", tc.ExpectedResponse.Error(), resp.Error())
			}
			if counter != tc.ExpectedNumOfCall {
				t.Fatalf("new: expected number of calls  '%d',  got '%d'", tc.ExpectedNumOfCall, counter)
			}

		})
	}
}

func TestPushUpdate(t *testing.T) {
	tcs := []struct {
		Name            string
		InputBranch     string
		InputRefName    string
		InputStatus     string
		ExpectedSuccess bool
	}{
		{
			Name:            "Should succeed",
			InputBranch:     "main",
			InputRefName:    "refs/heads/main",
			InputStatus:     "",
			ExpectedSuccess: true,
		},
		{
			Name:            "Should fail because wrong branch",
			InputBranch:     "main",
			InputRefName:    "refs/heads/master",
			InputStatus:     "",
			ExpectedSuccess: false,
		},
		{
			Name:            "Should fail because status not empty",
			InputBranch:     "master",
			InputRefName:    "refs/heads/master",
			InputStatus:     "i am the status, stopping this from working",
			ExpectedSuccess: false,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var success = false
			actualError := defaultPushUpdate(tc.InputBranch, &success)(tc.InputRefName, tc.InputStatus)
			if success != tc.ExpectedSuccess {
				t.Fatal(fmt.Sprintf("expected sucess=%t but got %t", tc.ExpectedSuccess, success))
			}
			if actualError != nil {
				t.Fatal(fmt.Sprintf("expected no error but got %s but got none", actualError))
			}
		})
	}
}

func TestDeleteDirIfEmpty(t *testing.T) {
	tcs := []struct {
		Name           string
		CreateThisDir  string
		DeleteThisDir  string
		ExpectedError  error
		ExpectedReason SuccessReason
	}{
		{
			Name:           "Should succeed: dir exists and is empty",
			CreateThisDir:  "foo/bar",
			DeleteThisDir:  "foo/bar",
			ExpectedReason: NoReason,
		},
		{
			Name:           "Should succeed: dir does not exist",
			CreateThisDir:  "foo/bar",
			DeleteThisDir:  "foo/bar/pow",
			ExpectedReason: DirDoesNotExist,
		},
		{
			Name:           "Should succeed: dir does not exist",
			CreateThisDir:  "foo/bar/pow",
			DeleteThisDir:  "foo/bar",
			ExpectedReason: DirNotEmpty,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			repo, _, _ := SetupRepositoryTestWithDB(t)
			state := repo.State()
			err := state.Filesystem.MkdirAll(tc.CreateThisDir, 0777)
			if err != nil {
				t.Fatalf("error in mkdir: %v", err)
				return
			}

			successReason, err := state.DeleteDirIfEmpty(tc.DeleteThisDir)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if successReason != tc.ExpectedReason {
				t.Fatal("Output mismatch (-want +got):\n", cmp.Diff(tc.ExpectedReason, successReason))
			}
		})
	}
}

func SetupRepositoryTestWithDB(t *testing.T) (Repository, *db.DBHandler, *RepositoryConfig) {
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
		t.Fatalf("error starting %v", err)
		return nil, nil, nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil, nil, nil
	}
	dbConfig.DbHost = dir
	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	config := RepositoryConfig{
		URL:                 "file://" + remoteDir,
		Path:                localDir,
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
		DBHandler:           dbHandler,
		Branch:              "master",
	}
	repo, err := New(
		testutil.MakeTestContext(),
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler, &config
}

func TestGetTagsNoTags(t *testing.T) {
	name := "No tags to be returned at all"

	t.Run(name, func(t *testing.T) {
		t.Parallel()

		_, _, repoConfig := SetupRepositoryTestWithDB(t)
		localDir := repoConfig.Path
		tags, err := GetTags(
			*repoConfig,
			localDir,
			testutil.MakeTestContext(),
		)
		if err != nil {
			t.Fatalf("new: expected no error, got '%e'", err)
		}
		if len(tags) != 0 {
			t.Fatalf("expected %v tags but got %v", 0, len(tags))
		}
	})

}

func TestGetTags(t *testing.T) {
	tcs := []struct {
		Name         string
		expectedTags []api.TagData
		tagsToAdd    []string
	}{
		{
			Name:         "Tags added to be returned",
			tagsToAdd:    []string{"v1.0.0"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v1.0.0", CommitId: ""}},
		},
		{
			Name:         "Tags added in opposite order and are sorted",
			tagsToAdd:    []string{"v1.0.1", "v0.0.1"},
			expectedTags: []api.TagData{{Tag: "refs/tags/v0.0.1", CommitId: ""}, {Tag: "refs/tags/v1.0.1", CommitId: ""}},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			_, _, repoConfig := SetupRepositoryTestWithDB(t)
			localDir := repoConfig.Path
			_, err := New(
				testutil.MakeTestContext(),
				*repoConfig,
			)
			if err != nil {
				t.Fatal(err)
			}
			repo, err := git.OpenRepository(localDir)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := repo.Index()
			if err != nil {
				t.Fatal(err)
			}
			treeId, err := idx.WriteTree()
			if err != nil {
				t.Fatal(err)
			}

			tree, err := repo.LookupTree(treeId)
			if err != nil {
				t.Fatal(err)
			}
			oid, err := repo.CreateCommit("HEAD", &git.Signature{Name: "SRE", Email: "testing@gmail"}, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing", tree)
			if err != nil {
				t.Fatal(err)
			}
			commit, err := repo.LookupCommit(oid)
			if err != nil {
				t.Fatal(err)
			}
			var expectedCommits []api.TagData
			for addTag := range tc.tagsToAdd {
				commit, err := repo.Tags.Create(tc.tagsToAdd[addTag], commit, &git.Signature{Name: "SRE", Email: "testing@gmail"}, "testing")
				expectedCommits = append(expectedCommits, api.TagData{Tag: tc.tagsToAdd[addTag], CommitId: commit.String()})
				if err != nil {
					t.Fatal(err)
				}
			}
			tags, err := GetTags(
				*repoConfig,
				localDir,
				testutil.MakeTestContext(),
			)
			if err != nil {
				t.Fatalf("new: expected no error, got '%e'", err)
			}
			if len(tags) != len(tc.expectedTags) {
				t.Fatalf("expected %v tags but got %v", len(tc.expectedTags), len(tags))
			}

			iter := 0
			for _, tagData := range tags {
				for commit := range expectedCommits {
					if tagData.Tag != expectedCommits[commit].Tag {
						if tagData.CommitId == expectedCommits[commit].CommitId {
							t.Fatalf("expected [%v] for TagList commit but got [%v]", expectedCommits[commit].CommitId, tagData.CommitId)
						}
					}
				}
				if tagData.Tag != tc.expectedTags[iter].Tag {
					t.Fatalf("expected [%v] for TagList tag but got [%v] with tagList %v", tc.expectedTags[iter].Tag, tagData.Tag, tags)
				}
				iter += 1
			}
		})
	}
}

func TestArgoCDFileGeneration(t *testing.T) {
	transformers := []Transformer{
		&CreateEnvironment{
			Environment: "production",
			Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "development",
				},
			}},
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest",
			},
			Version:             1,
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
		&CreateApplicationVersion{
			Application: "test",
			Manifests: map[string]string{
				"production": "manifest2",
			},
			Version:             2,
			TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
	}
	tcs := []struct {
		Name                string
		shouldGenerateFiles bool
	}{
		{
			Name:                "ArgoCD files should NOT be generated",
			shouldGenerateFiles: false,
		},
		{
			Name:                "Argo CD files should be generated",
			shouldGenerateFiles: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)
			repo.config.ArgoCdGenerateFiles = tc.shouldGenerateFiles
			state := repo.State()
			ctx := testutil.MakeTestContext()

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{Team: "test"})
				if err != nil {
					t.Fatalf("could not create app test: %v", err)
				}
				err = dbHandler.DBWriteEnvironment(ctx, transaction, "production", config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
					Destination: config.ArgoCdDestination{
						Server: "development",
					},
				}}, []string{"test"})
				if err != nil {
					t.Fatalf("could not create environment production: %v", err)
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production": "manifest2",
					}},
				})
				if err != nil {
					t.Fatalf("could not create release 1 for app test: %v", err)
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 2,
					App:           "test",
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production": "manifest2",
					}},
				})
				if err != nil {
					t.Fatalf("could not create release 1 for app test: %v", err)
				}
				for _, transformer := range transformers {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})

			state = repo.State() //update state

			if _, err := state.Filesystem.Stat("argocd"); errors.Is(err, os.ErrNotExist) {
				if tc.shouldGenerateFiles {
					t.Fatalf("Expected ArgoCD directory, but none was found. %v\n", err)
				}
			} else { //Argo CD dir exists
				if !tc.shouldGenerateFiles {
					t.Fatalf("ArgoCD files should not have been generated. Found ArgoCD directory.")
				}
			}

		})
	}
}

func TestMinimizeCommitsGeneration(t *testing.T) {
	var group = "dev"
	tcs := []struct {
		Name               string
		setup              []Transformer
		targetTransformers []Transformer
		shouldCreateCommit bool
		databasePopulation func(ctx context.Context, transaction *sql.Tx, handler *db.DBHandler) error
	}{
		{
			Name: "No-operation should not create new commits (Control)",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Delete environment Locks do not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentLock{
					Environment:           "production",
					LockId:                "my-lock",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return dbHandler.DBWriteEnvironmentLock(ctx, transaction, "my-lock", "production", db.LockMetadata{})
			},
		},
		{
			Name: "Create Environment Application Locks does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBWriteApplicationLock(ctx, transaction, "my-lock", "production", "test", db.LockMetadata{})
			},
		},
		{
			Name: "Delete Environment Application Locks does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentApplicationLock{
					Environment:           "production",
					LockId:                "my-lock",
					Application:           "test",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBWriteApplicationLock(ctx, transaction, "my-lock", "production", "test", db.LockMetadata{})
			},
		},
		{
			Name: "CreateEnvironmentTeamLock does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBWriteTeamLock(ctx, transaction, "my-lock", "production", "team-123", db.LockMetadata{})
			},
		},
		{
			Name: "DeleteEnvironmentTeamLock does not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: false}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					Message:               "my-lock-message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentTeamLock{
					Environment:           "production",
					LockId:                "my-lock",
					Team:                  "team-123",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBWriteTeamLock(ctx, transaction, "my-lock", "production", "team-123", db.LockMetadata{})
			},
		},
		{
			Name: "Migration transformer creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&MigrationTransformer{
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Delete Env From App creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvFromApp{
					Application:         "test",
					Environment:         "development",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
			},
		},
		{
			Name: "CreateUndeployApplicationVersion creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
			},
		},
		{
			Name: "CreateEnvironmentGroupLock does not create new commits",
			setup: []Transformer{

				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						EnvironmentGroup: &group,
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironmentGroupLock{
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "DeleteEnvironmentGroupLock does not create new commits",
			setup: []Transformer{

				&CreateEnvironment{
					Environment: "development",
					Config: config.EnvironmentConfig{
						EnvironmentGroup: &group,
					},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironmentGroupLock{
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "UndeployApplication creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             0,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&UndeployApplication{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
					Metadata: db.DBReleaseMetaData{
						UndeployVersion: true,
					},
				})
				return err
			},
		},
		{
			Name: "UndeployApplication creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironment{
					Environment:         "development",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					},
					Team:                "team-123",
					Version:             0,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateUndeployApplicationVersion{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&UndeployApplication{
					Application:         "test",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
				})
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production":  "manifest",
						"development": "manifest",
					}},
					Metadata: db.DBReleaseMetaData{
						UndeployVersion: true,
					},
				})
				return err
			},
		},
		{
			Name: "Create Application Version creates new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment: "production",
					Config: config.EnvironmentConfig{ArgoCd: &config.EnvironmentConfigArgoCd{
						Destination: config.ArgoCdDestination{
							Server: "development",
						},
					}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production": "manifest",
					}},
				})
			},
		},
		{
			Name: "Deployments should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team-123",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team-123",
				})
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production": "manifest",
					}},
				})
			},
		},
		{
			Name: "Create environment should not create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateEnvironment{
					Environment:         "staging",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: false,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return nil
			},
		},
		{
			Name: "Deleting environemnts should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&DeleteEnvironment{
					Environment:         "production",
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				return dbHandler.DBWriteEnvironment(ctx, transaction, "production", config.EnvironmentConfig{}, []string{})
			},
		},
		{
			Name: "Mixed should create new commits",
			setup: []Transformer{
				&CreateEnvironment{
					Environment:         "production",
					Config:              config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}},
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			targetTransformers: []Transformer{
				&CreateApplicationVersion{
					Application: "test",
					Manifests: map[string]string{
						"production": "manifest",
					},
					Team:                "team",
					Version:             1,
					TransformerMetadata: TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
				&CreateEnvironmentLock{
					Environment:           "production",
					LockId:                "my-lock",
					Message:               "my lock message",
					TransformerEslVersion: 0,
					TransformerMetadata:   TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
				},
			},
			shouldCreateCommit: true,
			databasePopulation: func(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
				err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, "test", db.AppStateChangeCreate, db.DBAppMetaData{
					Team: "team",
				})
				if err != nil {
					return err
				}
				err = dbHandler.DBWriteEnvironmentLock(ctx, transaction, "my-lock", "production", db.LockMetadata{})
				if err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumber: 1,
					App:           "test",
					Environments:  []string{"production"},
					Manifests: db.DBReleaseManifests{Manifests: map[string]string{
						"production": "manifest",
					}},
				})
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			r, dbHandler, _ := SetupRepositoryTestWithDB(t)
			repo := r.(*repository)

			ctx := testutil.MakeTestContext()

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := tc.databasePopulation(ctx, transaction, dbHandler)
				if err != nil {
					t.Fatalf("Unexpected error populating the database. Error: %v", err)
				}
				for _, transformer := range tc.setup {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})
			repo.config.MinimizeExportedData = true

			initialCommitId, err := repo.GetHeadCommitId()
			if err != nil {
				t.Fatalf("Could not retrieve commit: Error: %v", err)
			}
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range tc.targetTransformers {
					_, applyErr := repo.ApplyTransformer(ctx, transaction, transformer)
					if applyErr != nil && applyErr.TransformerError != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})
			finalCommitId, err := repo.GetHeadCommitId()
			if err != nil {
				t.Fatalf("Could not retrieve commit: Error: %v", err)
			}
			if initialCommitId.String() == finalCommitId.String() == tc.shouldCreateCommit {
				t.Fatalf("commit check failed. commits same: %v want: %v", initialCommitId.String() == finalCommitId.String(), tc.shouldCreateCommit)
			}
		})
	}
}
