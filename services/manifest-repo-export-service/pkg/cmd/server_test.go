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

package cmd

import (
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/config"
	errs "github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/service"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
)

func TestCalculateProcessDelay(t *testing.T) {
	exampleTime, err := time.Parse("2006-01-02 15:04:05", "2024-06-18 16:14:07")
	if err != nil {
		t.Fatal(err)
	}
	exampleTime10SecondsBefore := exampleTime.Add(-10 * time.Second)
	tcs := []struct {
		Name          string
		eslEvent      *db.EslEventRow
		currentTime   time.Time
		ExpectedDelay float64
	}{
		{
			Name:          "Should return 0 if there are no events",
			eslEvent:      nil,
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name:          "Should return 0 if time created is not set",
			eslEvent:      &db.EslEventRow{},
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name: "With one Event",
			eslEvent: &db.EslEventRow{
				EslVersion: 1,
				Created:    exampleTime10SecondsBefore,
				EventType:  "CreateApplicationVersion",
				EventJson:  "{}",
			},
			currentTime:   exampleTime,
			ExpectedDelay: 10,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutil.MakeTestContext()
			delay, err := calculateProcessDelay(ctx, tc.eslEvent, tc.currentTime)
			if err != nil {
				t.Fatal(err)
			}
			if delay != tc.ExpectedDelay {
				t.Errorf("expected %f, got %f", tc.ExpectedDelay, delay)
			}
		})
	}
}

func TestPushGitTags(t *testing.T) {
	var setup = makeSetupTransformer()
	tcs := []struct {
		Name                   string
		tagsToAdd              []types.GitTag
		overwriteTagsPath      bool
		failOnErrorWithGitTags bool
		expectedPushError      error
		expectedGetTagsError   error
		expectedTags           *api.GetGitTagsResponse
	}{
		{
			Name:                   "Single Tag is returned",
			tagsToAdd:              []types.GitTag{"moin"},
			overwriteTagsPath:      false,
			failOnErrorWithGitTags: true,
			expectedPushError:      nil,
			expectedGetTagsError:   nil,
			expectedTags: &api.GetGitTagsResponse{
				TagData: []*api.TagData{
					{
						Tag:        "refs/tags/moin",
						CommitId:   "", // ignored
						CommitDate: nil,
					},
				},
			},
		},
		{
			Name:                   "Invalid tags path",
			tagsToAdd:              []types.GitTag{"moin"},
			overwriteTagsPath:      true,
			failOnErrorWithGitTags: true,
			expectedPushError:      nil,
			expectedGetTagsError: errs.ErrMatcher{
				Message: "tagsPath must not be empty",
			},
			expectedTags: nil,
		},
		{
			Name:                   "Invalid tags path, but ignore error=true",
			tagsToAdd:              []types.GitTag{"moin"},
			overwriteTagsPath:      true,
			failOnErrorWithGitTags: false,
			expectedPushError:      nil,
			expectedGetTagsError: errs.ErrMatcher{
				Message: "tagsPath must not be empty",
			},
			expectedTags: nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			repo, dbHandler, repoConfig := SetupRepositoryTestWithDB(t, ctx)
			if tc.overwriteTagsPath {
				repoConfig.TagsPath = ""
			}

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range setup {
					applyErr := repo.Apply(ctx, transaction, transformer)
					if applyErr != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
				}
				return nil
			})

			for _, tagToAdd := range tc.tagsToAdd {
				actualErr := HandleGitTagPush(ctx, repo, tagToAdd, nil, tc.failOnErrorWithGitTags)
				if diff := cmp.Diff(tc.expectedPushError, actualErr, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
			}

			sv := &service.GitServer{
				Repository: repo,
				Config:     *repoConfig,
				PageSize:   uint64(100),
				DBHandler:  dbHandler,
			}

			actualTags, actualErr := sv.GetGitTags(ctx, nil)
			if diff := cmp.Diff(tc.expectedGetTagsError, actualErr, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.expectedTags, actualTags, protocmp.Transform(), protocmp.IgnoreFields(&api.TagData{}, "commit_id")); diff != "" {
				t.Fatalf("tags mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestHandleOneEvent(t *testing.T) {
	var setup = makeSetupTransformer()
	type CutoffData struct {
		eventType db.EventType
		data      interface{}
		metadata  db.ESLMetadata
	}
	tcs := []struct {
		Name                string
		withCutoff          *CutoffData
		expectedError       error
		expectedTransformer repository.Transformer
		expectedRow         *db.EslEventRow
	}{
		{
			Name:                "does nothing when there is no read cutoff",
			withCutoff:          nil,
			expectedError:       nil,
			expectedTransformer: nil,
			expectedRow:         nil,
		},
		{
			Name: "finds the cutoff",
			withCutoff: &CutoffData{
				eventType: db.EvtCreateApplicationVersion,
				data:      nil,
				metadata:  db.ESLMetadata{},
			},
			expectedError:       nil,
			expectedTransformer: nil,
			expectedRow:         nil,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			repo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)
			//const eslVersion = 0

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range setup {
					applyErr := repo.Apply(ctx, transaction, transformer)
					if applyErr != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
					if tc.withCutoff != nil {
						err := dbHandler.DBWriteEslEventInternal(ctx, tc.withCutoff.eventType, transaction, tc.withCutoff.data, tc.withCutoff.metadata)
						if err != nil {
							t.Fatalf("Unexpected error inserting event: Error: %v", err)
						}
						eventRow, err := dbHandler.DBReadEslEventInternal(ctx, transaction, true)
						if err != nil {
							t.Fatalf("Unexpected error inserting event: Error: %v", err)
						}
						err = db.DBWriteCutoff(dbHandler, ctx, transaction, eventRow.EslVersion)
						if err != nil {
							t.Fatalf("Unexpected error inserting cutoff: Error: %v", err)
						}
					}
				}
				return nil
			})
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				actualTransformer, actualRow, actualError := HandleOneEvent(ctx, transaction, dbHandler, nil, repo)
				if diff := cmp.Diff(tc.expectedError, actualError, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.expectedTransformer, actualTransformer); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.expectedRow, actualRow); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}

				return nil
			})

		})
	}
}

func makeSetupTransformer() []repository.Transformer {
	return []repository.Transformer{
		&repository.CreateEnvironment{
			Environment: "production",
			Config: config.EnvironmentConfig{Upstream: &config.EnvironmentConfigUpstream{Latest: true}, ArgoCd: &config.EnvironmentConfigArgoCd{
				Destination: config.ArgoCdDestination{
					Server: "development",
				},
			}},
			TransformerMetadata: repository.TransformerMetadata{AuthorName: "test", AuthorEmail: "testmail@example.com"},
		},
	}
}

func SetupRepositoryTestWithDB(t *testing.T, ctx context.Context) (repository.Repository, *db.DBHandler, *repository.RepositoryConfig) {
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	t.Logf("remote dir: %s", remoteDir)
	//t.Logf("local  dir: %s", localDir)
	cmd := exec.Command("git", "init", "--bare", remoteDir, "--initial-branch=master")
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

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	config := repository.RepositoryConfig{
		URL:                 "file://" + remoteDir,
		Path:                localDir,
		TagsPath:            localDir, // in a test we can use the same directory for tags and other operations
		CommitterEmail:      "kuberpult@freiheit.com",
		CommitterName:       "kuberpult",
		ArgoCdGenerateFiles: true,
		DBHandler:           dbHandler,
		Branch:              "master",
	}
	repo, err := repository.New(
		testutil.MakeTestContext(),
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler, &config
}
