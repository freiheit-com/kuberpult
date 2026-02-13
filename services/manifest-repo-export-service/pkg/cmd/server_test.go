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
	"errors"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/argocd"
	"github.com/freiheit-com/kuberpult/pkg/backoff"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	errs "github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/service"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	"os/exec"
	"path"
	"testing"
	"time"
)

func TestParseEnvironmentOverrides(t *testing.T) {
	tcs := []struct {
		Name string

		ConfiguredOverrides valid.StringMap
		DbEnvironments      []types.EnvName

		ExpectedArgoProjectNamesPerEnv *argocd.ArgoProjectNamesPerEnv
	}{
		{
			Name:                           "empty input results in empty map",
			ConfiguredOverrides:            map[string]string{},
			DbEnvironments:                 []types.EnvName{"dev"},
			ExpectedArgoProjectNamesPerEnv: &argocd.ArgoProjectNamesPerEnv{},
		},
		{
			Name: "env is missing",
			ConfiguredOverrides: map[string]string{
				"fake-env": "argo-proj-1",
			},
			DbEnvironments:                 []types.EnvName{"dev"},
			ExpectedArgoProjectNamesPerEnv: &argocd.ArgoProjectNamesPerEnv{},
		},
		{
			Name: "1 valid env override",
			ConfiguredOverrides: map[string]string{
				"dev": "argo-proj-2",
			},
			DbEnvironments: []types.EnvName{"dev"},
			ExpectedArgoProjectNamesPerEnv: &argocd.ArgoProjectNamesPerEnv{
				"dev": "argo-proj-2",
			},
		},
		{
			Name: "1 valid + 1 invalid env override",
			ConfiguredOverrides: map[string]string{
				"dev": "argo-proj-dev",
				"prd": "argo-proj-prd",
			},
			DbEnvironments: []types.EnvName{"prd"},
			ExpectedArgoProjectNamesPerEnv: &argocd.ArgoProjectNamesPerEnv{
				"prd": "argo-proj-prd",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			actual := ParseEnvironmentOverrides(ctx, tc.ConfiguredOverrides, tc.DbEnvironments)

			if diff := testutil.CmpDiff(actual, tc.ExpectedArgoProjectNamesPerEnv); diff != "" {
				t.Logf("actual configuration: %v", actual)
				t.Logf("expected configuration: %v", tc.ExpectedArgoProjectNamesPerEnv)
				t.Errorf("expected args:\n  %v\ngot:\n  %v\ndiff:\n  %s\n", actual, tc.ExpectedArgoProjectNamesPerEnv, diff)
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

func TestHandleOneTransformer(t *testing.T) {
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
				actualTransformer, actualRow, actualError := HandleOneTransformer(ctx, transaction, dbHandler, nil, repo)
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
		testutilauth.MakeTestContext(),
		config,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler, &config
}

func TestProcessOneEvent(t *testing.T) {
	const (
		minSleep = time.Nanosecond * 1
		maxSleep = time.Nanosecond * 1000

		app = "test-app"
		env = "dev"
	)
	var initialSyncData *db.GitSyncData = &db.GitSyncData{
		AppName:       app,
		EnvName:       env,
		TransformerID: 1,
		SyncStatus:    db.UNSYNCED,
	}
	var setup = makeSetupTransformer()
	type CutoffData struct {
		eventType db.EventType
		data      interface{}
		metadata  db.ESLMetadata
	}
	tcs := []struct {
		Name                  string
		withCutoff            *CutoffData
		expectedError         error
		expectedTransformer   repository.Transformer
		expectedSleepDuration time.Duration
		expectedSyncStatus    []GitSyncStatusRow
		expectNotification    bool
	}{
		{
			Name:                  "does nothing when there is no read cutoff",
			withCutoff:            nil,
			expectedError:         nil,
			expectedTransformer:   nil,
			expectedSleepDuration: 2,
			expectedSyncStatus:    []GitSyncStatusRow{},
			expectNotification:    false,
		},
		{
			Name: "process one transformer",
			withCutoff: &CutoffData{
				eventType: db.EvtCreateApplicationVersion,
				data:      nil,
				metadata: db.ESLMetadata{
					AuthorName:  "author one",
					AuthorEmail: "email two",
				},
			},
			expectedError:         nil,
			expectedTransformer:   nil,
			expectedSleepDuration: 0,
			expectedSyncStatus: []GitSyncStatusRow{
				{
					TransformerId: 1,
					EnvName:       env,
					AppName:       app,
					Status:        db.SYNCED,
				},
			},
			expectNotification: true,
		},
		{
			Name: "fail HandleOneTransformer",
			withCutoff: &CutoffData{
				eventType: "foobar-does-not-exist",
				data:      nil,
				metadata: db.ESLMetadata{
					AuthorName:  "author one",
					AuthorEmail: "email two",
				},
			},
			expectedError:         nil,
			expectedTransformer:   nil,
			expectedSleepDuration: 0,
			expectedSyncStatus: []GitSyncStatusRow{
				{
					TransformerId: 1,
					EnvName:       env,
					AppName:       app,
					Status:        db.SYNC_FAILED,
				},
			},
			expectNotification: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			repo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)
			var transformerId db.TransformerID

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range setup {
					applyErr := repo.Apply(ctx, transaction, transformer)
					if applyErr != nil {
						t.Fatalf("Unexpected error applying transformers: Error: %v", applyErr)
					}
					if tc.withCutoff != nil {
						err := dbHandler.DBWriteEslEventInternal(ctx, tc.withCutoff.eventType, transaction, tc.withCutoff.data, tc.withCutoff.metadata)
						if err != nil {
							t.Fatalf("DBWriteEslEventInternal Error: %v", err)
						}
						event, err := dbHandler.DBReadEslEventInternal(ctx, transaction, true)
						if err != nil {
							t.Fatalf("DBReadEslEventInternal Error: %v", err)
						}
						transformerId = db.TransformerID(event.EslVersion)
						err = dbHandler.DBWriteNewSyncEvent(ctx, transaction, initialSyncData)
						if err != nil {
							t.Fatalf("DBReadEslEventInternal Error: %v", err)
						}
					}
				}
				return nil
			})

			subChannel, unsubFunc := repo.Notify().Subscribe()
			defer unsubFunc()
			<-subChannel // there's always one element in the channel, so we remove it here

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				sleepDuration := backoff.MakeSimpleBackoff(minSleep, maxSleep)
				actualSleepDuration, actualError := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true)
				if diff := cmp.Diff(tc.expectedError, actualError, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.expectedSleepDuration, actualSleepDuration); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}

				allActualRows, err := DBReadGitSyncStatusAll(ctx, dbHandler, transaction, transformerId)
				if err != nil {
					t.Fatalf("DBReadGitSyncStatusAll Error: %v", err)
				}
				if diff := cmp.Diff(tc.expectedSyncStatus, allActualRows); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}

				// Test that the notification works
				select {
				case _, ok := <-subChannel:
					if ok {
						if tc.expectNotification {
							return nil // everything is fine
						}
						t.Fatalf("expected no notification, but got one!")
					} else {
						t.Fatalf("Channel closed! (else)")
					}
				default:
					if tc.expectNotification {
						t.Fatalf("expected a notification, but got none!")
					}
				}
				return nil
			})

		})
	}
}

type GitSyncStatusRow struct {
	TransformerId db.TransformerID
	EnvName       types.EnvName
	AppName       types.AppName
	Status        db.SyncStatus
}

func DBReadGitSyncStatusAll(ctx context.Context, h *db.DBHandler, tx *sql.Tx, id db.TransformerID) (_ []GitSyncStatusRow, err error) {
	selectQuery := h.AdaptQuery(`
		SELECT transformerid, envName, appName, status
		FROM git_sync_status
		WHERE transformerid = ?
		ORDER BY created DESC;`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get current eslVersion. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	allCombinations := make([]GitSyncStatusRow, 0)
	var oneRow GitSyncStatusRow
	for rows.Next() {
		err := rows.Scan(&oneRow.TransformerId, &oneRow.EnvName, &oneRow.AppName, &oneRow.Status)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error table for next eslVersion. Error: %w", err)
		}
		allCombinations = append(allCombinations, oneRow)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("row has error: %v", err)
	}
	return allCombinations, nil
}
