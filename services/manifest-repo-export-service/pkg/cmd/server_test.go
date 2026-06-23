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
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/backoff"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	errs "github.com/freiheit-com/kuberpult/pkg/errorMatcher"
	"github.com/freiheit-com/kuberpult/pkg/logging"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/argocd"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/service"
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

func TestHandleBatchEvents(t *testing.T) {
	var setup = makeSetupTransformer()
	type CutoffData struct {
		eventType db.EventType
		data      interface{}
		metadata  db.ESLMetadata
	}
	tcs := []struct {
		Name                 string
		withCutoff           *CutoffData
		expectedError        error
		expectedTransformers []repository.Transformer
		expectedRows         []*db.EslEventRow
	}{
		{
			Name:                 "does nothing when there is no read cutoff",
			withCutoff:           nil,
			expectedError:        nil,
			expectedTransformers: nil,
			expectedRows:         nil,
		},
		{
			Name: "finds the cutoff",
			withCutoff: &CutoffData{
				eventType: db.EvtCreateApplicationVersion,
				data:      nil,
				metadata:  db.ESLMetadata{},
			},
			expectedError:        nil,
			expectedTransformers: nil,
			expectedRows:         nil,
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
				actualBatch, actualError := HandleBatchEvents(ctx, transaction, dbHandler, nil, repo, 1)
				if diff := cmp.Diff(tc.expectedError, actualError, cmpopts.EquateErrors()); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				var actualTransformers []repository.Transformer
				var actualRows []*db.EslEventRow
				for _, b := range actualBatch {
					actualTransformers = append(actualTransformers, b.Transformer)
					actualRows = append(actualRows, b.Esl)
				}
				if diff := cmp.Diff(tc.expectedTransformers, actualTransformers); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.expectedRows, actualRows); diff != "" {
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
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
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
		ArgoRenderOptions:   &argocd.RenderOptions{RenderApps: true},
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
			Name: "fail HandleBatchEvents",
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
				actualSleepDuration, actualError := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, 1)
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
			logging.Error(ctx, "row closing error.", zap.Error(err))
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

// makeEslEvents builds esl event rows with consecutive eslVersions starting at 1 and the given
// types. Only the version and type are relevant for selectBatch, so nothing else is set.
func makeEslEvents(types ...db.EventType) []*db.EslEventRow {
	rows := make([]*db.EslEventRow, 0, len(types))
	for i, t := range types {
		//exhaustruct:ignore
		rows = append(rows, &db.EslEventRow{
			EslVersion: db.EslVersion(i + 1),
			EventType:  t,
		})
	}
	return rows
}

// batchShape is the (version, type) projection of a batch, used so assertions compare Go objects.
type batchShape struct {
	EslVersion db.EslVersion
	EventType  db.EventType
}

func shapeOf(rows []*db.EslEventRow) []batchShape {
	out := make([]batchShape, 0, len(rows))
	for _, r := range rows {
		out = append(out, batchShape{EslVersion: r.EslVersion, EventType: r.EventType})
	}
	return out
}

func TestSelectBatch(t *testing.T) {
	const create = db.EvtCreateApplicationVersion
	const deploy = db.EvtDeployApplicationVersion
	const undeploy = db.EvtUndeployApplication
	tcs := []struct {
		Name         string
		Input        []db.EventType
		MaxBatchSize int
		Expected     []batchShape
	}{
		{
			Name:         "empty input yields empty batch",
			Input:        nil,
			MaxBatchSize: 5,
			Expected:     []batchShape{},
		},
		{
			Name:         "single CreateApplicationVersion yields batch of 1",
			Input:        []db.EventType{create},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, create}},
		},
		{
			Name:         "single non-Create yields batch of 1",
			Input:        []db.EventType{deploy},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, deploy}},
		},
		{
			Name:         "run of creates shorter than max yields full run",
			Input:        []db.EventType{create, create, create},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, create}, {2, create}, {3, create}},
		},
		{
			Name:         "run longer than max is capped at max",
			Input:        []db.EventType{create, create, create, create},
			MaxBatchSize: 2,
			Expected:     []batchShape{{1, create}, {2, create}},
		},
		{
			Name:         "stops at first non-Create (Create,Create,Undeploy,Create)",
			Input:        []db.EventType{create, create, undeploy, create},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, create}, {2, create}},
		},
		{
			Name:         "screenshot example Create,Undeploy,Create yields batch of 1",
			Input:        []db.EventType{create, undeploy, create},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, create}},
		},
		{
			Name:         "leading non-Create yields batch of 1 (never reorders/skips)",
			Input:        []db.EventType{undeploy, create, create},
			MaxBatchSize: 5,
			Expected:     []batchShape{{1, undeploy}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			batch := selectBatch(makeEslEvents(tc.Input...), tc.MaxBatchSize)
			actual := shapeOf(batch)
			if diff := testutil.CmpDiff(tc.Expected, actual); diff != "" {
				t.Errorf("batch mismatch (-want, +got):\n%s", diff)
			}
			// Invariant: returned versions are strictly contiguous and ascending.
			for i := 1; i < len(actual); i++ {
				if actual[i].EslVersion != actual[i-1].EslVersion+1 {
					t.Errorf("batch not strictly contiguous at index %d: %v", i, actual)
				}
			}
		})
	}
}

func TestSelectBatchMaxSizeBoundary(t *testing.T) {
	const create = db.EvtCreateApplicationVersion
	tcs := []struct {
		Name         string
		MaxBatchSize int
	}{
		{Name: "maxBatchSize 0 treated as 1", MaxBatchSize: 0},
		{Name: "maxBatchSize 1 yields batch of 1", MaxBatchSize: 1},
		{Name: "negative maxBatchSize treated as 1", MaxBatchSize: -3},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			// three contiguous creates; with an effective max of 1, only the first is taken.
			batch := selectBatch(makeEslEvents(create, create, create), tc.MaxBatchSize)
			if len(batch) != 1 {
				t.Errorf("expected batch of 1 (== today's behavior), got %d", len(batch))
			}
		})
	}
}

func TestBuildTransformerBatch(t *testing.T) {
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	// buildTransformer sets EslVersion/CreationTimestamp from the row and resolves the concrete
	// transformer type from the event type; we project to those observable properties so the
	// assertion compares Go objects rather than unexported transformer internals.
	type transformerShape struct {
		GoType     string
		EventType  db.EventType
		EslVersion db.TransformerID
		Created    time.Time
	}
	tcs := []struct {
		Name     string
		Input    []db.EventType
		Expected []transformerShape
	}{
		{
			Name:  "batch of CreateApplicationVersion events",
			Input: []db.EventType{db.EvtCreateApplicationVersion, db.EvtCreateApplicationVersion},
			Expected: []transformerShape{
				{"*repository.CreateApplicationVersion", db.EvtCreateApplicationVersion, 1, created},
				{"*repository.CreateApplicationVersion", db.EvtCreateApplicationVersion, 2, created},
			},
		},
		{
			Name:  "mixed types build the matching concrete type in order",
			Input: []db.EventType{db.EvtDeployApplicationVersion, db.EvtCreateEnvironmentLock},
			Expected: []transformerShape{
				{"*repository.DeployApplicationVersion", db.EvtDeployApplicationVersion, 1, created},
				{"*repository.CreateEnvironmentLock", db.EvtCreateEnvironmentLock, 2, created},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			actual := make([]transformerShape, 0, len(tc.Input))
			for i, et := range tc.Input {
				//exhaustruct:ignore
				row := &db.EslEventRow{
					EslVersion: db.EslVersion(i + 1),
					EventType:  et,
					EventJson:  "{}",
					Created:    created,
				}
				transformer, err := buildTransformer(ctx, row)
				if err != nil {
					t.Fatalf("buildTransformer error for %s: %v", et, err)
				}
				actual = append(actual, transformerShape{
					GoType:     fmt.Sprintf("%T", transformer),
					EventType:  transformer.GetDBEventType(),
					EslVersion: transformer.GetEslVersion(),
					Created:    transformer.GetCreationTimestamp(),
				})
			}
			if diff := testutil.CmpDiff(tc.Expected, actual); diff != "" {
				t.Errorf("transformer construction mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// TestProcessOneEventBatchFailureFallback verifies that when a batch fails to apply (here the 2nd of
// three CreateApplicationVersion events has invalid JSON), ProcessOneEvent falls back to processing
// one event at a time: the preceding good event still processes, the offending event lands in
// esl_failed_events with the cutoff advancing past it, and the trailing event is not poisoned
// (Task 5 / R-2).
func TestProcessOneEventBatchFailureFallback(t *testing.T) {
	const minSleep = time.Nanosecond * 1
	const maxSleep = time.Nanosecond * 1000
	tcs := []struct {
		Name                  string
		maxBatchSize          int
		expectedFinalCutoff   db.EslVersion
		expectedFailedVersion db.EslVersion
	}{
		{
			Name:                  "second event in a batch of three fails and is isolated",
			maxBatchSize:          3,
			expectedFinalCutoff:   3,
			expectedFailedVersion: 2,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			repo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)

			// Apply the environment setup directly (as the cd-service would), then seed three
			// CreateApplicationVersion esl events. The second one carries invalid JSON so building
			// its transformer fails during apply.
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, transformer := range makeSetupTransformer() {
					if applyErr := repo.Apply(ctx, transaction, transformer); applyErr != nil {
						t.Fatalf("Unexpected error applying setup transformers: %v", applyErr)
					}
				}
				meta := db.ESLMetadata{AuthorName: "author", AuthorEmail: "email"}
				if err := dbHandler.DBWriteEslEventInternal(ctx, db.EvtCreateApplicationVersion, transaction, interface{}(nil), meta); err != nil {
					t.Fatalf("seeding event 1: %v", err)
				}
				if err := dbHandler.DBWriteEslEventWithJson(ctx, transaction, db.EvtCreateApplicationVersion, `{ this is not valid json`); err != nil {
					t.Fatalf("seeding event 2: %v", err)
				}
				if err := dbHandler.DBWriteEslEventInternal(ctx, db.EvtCreateApplicationVersion, transaction, interface{}(nil), meta); err != nil {
					t.Fatalf("seeding event 3: %v", err)
				}
				return nil
			})

			// Drive the loop a few times; three events means at most three successful iterations.
			sleepDuration := backoff.MakeSimpleBackoff(minSleep, maxSleep)
			for i := 0; i < 5; i++ {
				if _, err := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, tc.maxBatchSize); err != nil {
					t.Fatalf("ProcessOneEvent iteration %d returned error: %v", i, err)
				}
			}

			_ = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				cutoff, err := db.DBReadCutoff(dbHandler, ctx, transaction)
				if err != nil {
					t.Fatalf("DBReadCutoff: %v", err)
				}
				if cutoff == nil || *cutoff != tc.expectedFinalCutoff {
					t.Errorf("expected final cutoff %d, got %v", tc.expectedFinalCutoff, cutoff)
				}
				failed, err := dbHandler.DBReadLastFailedEslEvents(ctx, transaction, 10, 0)
				if err != nil {
					t.Fatalf("DBReadLastFailedEslEvents: %v", err)
				}
				gotFailed := make([]db.EslVersion, 0, len(failed))
				for _, f := range failed {
					gotFailed = append(gotFailed, f.TransformerEslVersion)
				}
				if diff := testutil.CmpDiff([]db.EslVersion{tc.expectedFailedVersion}, gotFailed); diff != "" {
					t.Errorf("failed-event versions mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
		})
	}
}

// pushCountingRepo wraps a Repository to count how many times PushRepo is called, so a test can
// assert a batch results in exactly one push.
type pushCountingRepo struct {
	repository.Repository
	pushCount int
}

func (p *pushCountingRepo) PushRepo(ctx context.Context) error {
	p.pushCount++
	return p.Repository.PushRepo(ctx)
}

// seedCommittingCreateApp writes a CreateApplicationVersion esl event plus the DB state (app,
// release, and — when withDeployment is true — a deployment keyed to the event's eslVersion) needed
// for the export to actually deploy and commit when it replays the event. Without a deployment the
// event replays as a NoOp (R-3). Events get sequential esl versions, so call this in order. Returns
// the assigned eslVersion.
func seedCommittingCreateApp(ctx context.Context, t *testing.T, dbHandler *db.DBHandler, tx *sql.Tx, app types.AppName, env types.EnvName, withDeployment bool) db.EslVersion {
	const author = "author"
	const email = "email@example.com"
	//exhaustruct:ignore
	cav := &repository.CreateApplicationVersion{
		Application: string(app),
		Manifests:   map[types.EnvName]string{env: "manifest-" + string(app)},
		Version:     1,
	}
	if err := dbHandler.DBWriteEslEventInternal(ctx, db.EvtCreateApplicationVersion, tx, cav, db.ESLMetadata{AuthorName: author, AuthorEmail: email}); err != nil {
		t.Fatalf("write esl event: %v", err)
	}
	row, err := dbHandler.DBReadEslEventInternal(ctx, tx, false) // false => last (highest) row
	if err != nil {
		t.Fatalf("read esl event: %v", err)
	}
	eslVersion := row.EslVersion
	if err := dbHandler.DBInsertOrUpdateApplication(ctx, tx, app, db.AppStateChangeCreate, db.DBAppMetaData{Team: "team"}, types.ArgoBracketName(app)); err != nil {
		t.Fatalf("insert app: %v", err)
	}
	v := uint64(1)
	if err := dbHandler.DBUpdateOrCreateRelease(ctx, tx, db.DBReleaseWithMetaData{
		ReleaseNumbers: types.ReleaseNumbers{Version: &v, Revision: 0},
		App:            app,
		Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{env: "manifest-" + string(app)}},
	}); err != nil {
		t.Fatalf("create release: %v", err)
	}
	if withDeployment {
		if err := dbHandler.DBUpdateOrCreateDeployment(ctx, tx, db.Deployment{
			App:            app,
			Env:            env,
			ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
			Metadata:       db.DeploymentMetadata{DeployedByEmail: email, DeployedByName: author},
			TransformerID:  db.TransformerID(eslVersion),
		}); err != nil {
			t.Fatalf("create deployment: %v", err)
		}
	}
	return eslVersion
}

// TestProcessOneEventBatchedPushEndToEnd drives ProcessOneEvent over a batch of adjacent
// CreateApplicationVersion events and asserts the batch is collapsed into a single push, the cutoff
// advances to the highest version, the expected number of commits are created (NoOp events produce
// none, R-3) and every batched app ends up SYNCED (Tasks 11 / 11b).
func TestProcessOneEventBatchedPushEndToEnd(t *testing.T) {
	const minSleep = time.Nanosecond * 1
	const maxSleep = time.Nanosecond * 1000
	const env = types.EnvName("production")
	tcs := []struct {
		Name            string
		deploys         []bool // one entry per app in the batch; true => deploys (commits), false => NoOp
		expectedCommits int
		expectedCutoff  db.EslVersion
		expectedPushes  int
	}{
		{
			Name:            "batch of three releases, one push, three commits",
			deploys:         []bool{true, true, true},
			expectedCommits: 3,
			expectedCutoff:  3,
			expectedPushes:  1,
		},
		{
			Name:            "NoOp in the middle still one push, two commits",
			deploys:         []bool{true, false, true},
			expectedCommits: 2,
			expectedCutoff:  3,
			expectedPushes:  1,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			baseRepo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)
			repo := &pushCountingRepo{Repository: baseRepo}

			envConfig := config.EnvironmentConfig{
				Upstream: &config.EnvironmentConfigUpstream{Latest: true},
				ArgoCd:   &config.EnvironmentConfigArgoCd{Destination: config.ArgoCdDestination{Server: "prod-server"}},
			}
			// Seed the env (DB) and the batch's apps/releases/deployments, then apply the env to git
			// so there is a base commit for the batch to build on.
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteEnvironment(ctx, transaction, env, envConfig); err != nil {
					t.Fatalf("write env: %v", err)
				}
				for i, deploy := range tc.deploys {
					seedCommittingCreateApp(ctx, t, dbHandler, transaction, types.AppName(fmt.Sprintf("app-%d", i+1)), env, deploy)
				}
				return nil
			})
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				return baseRepo.Apply(ctx, transaction, &repository.CreateEnvironment{
					Environment:         env,
					Config:              envConfig,
					TransformerMetadata: repository.TransformerMetadata{AuthorName: "author", AuthorEmail: "email@example.com"},
				})
			})

			sleepDuration := backoff.MakeSimpleBackoff(minSleep, maxSleep)
			if _, err := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, len(tc.deploys)); err != nil {
				t.Fatalf("ProcessOneEvent: %v", err)
			}

			if repo.pushCount != tc.expectedPushes {
				t.Errorf("expected %d push(es), got %d", tc.expectedPushes, repo.pushCount)
			}
			// One commit_transaction_timestamps row is written per commit (Task 4 / Task 14), so the
			// row count is the number of commits the batch produced (NoOp events produce none, R-3).
			if got := countCommitTimestamps(ctx, t, dbHandler); got != tc.expectedCommits {
				t.Errorf("expected %d commit-timestamp rows (= commits), got %d", tc.expectedCommits, got)
			}
			_ = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				cutoff, err := db.DBReadCutoff(dbHandler, ctx, transaction)
				if err != nil {
					t.Fatalf("read cutoff: %v", err)
				}
				if cutoff == nil || *cutoff != tc.expectedCutoff {
					t.Errorf("expected cutoff %d, got %v", tc.expectedCutoff, cutoff)
				}
				return nil
			})
		})
	}
}

// countCommitTimestamps returns the number of rows in commit_transaction_timestamps, which equals
// the number of commits the export has produced (one row written per commit).
func countCommitTimestamps(ctx context.Context, t *testing.T, dbHandler *db.DBHandler) int {
	t.Helper()
	count := 0
	if err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		row := transaction.QueryRowContext(ctx, "SELECT COUNT(*) FROM commit_transaction_timestamps;")
		return row.Scan(&count)
	}); err != nil {
		t.Fatalf("count commit_transaction_timestamps: %v", err)
	}
	return count
}

// failingPushRepo wraps a Repository and makes PushRepo fail while fail is true, to exercise the
// push-failure path. It also counts push attempts.
type failingPushRepo struct {
	repository.Repository
	fail      bool
	pushCount int
}

func (f *failingPushRepo) PushRepo(ctx context.Context) error {
	f.pushCount++
	if f.fail {
		return fmt.Errorf("simulated push failure")
	}
	return f.Repository.PushRepo(ctx)
}

// TestProcessOneEventBatchPushFailureReprocesses asserts that when a batch's push fails the cutoff
// does NOT advance, and once the push recovers the whole batch reprocesses cleanly (Task 14b / R-6).
func TestProcessOneEventBatchPushFailureReprocesses(t *testing.T) {
	const minSleep = time.Nanosecond * 1
	const maxSleep = time.Nanosecond * 1000
	const env = types.EnvName("production")
	ctx := context.Background()
	baseRepo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)
	repo := &failingPushRepo{Repository: baseRepo, fail: true}

	envConfig := config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{Latest: true},
		ArgoCd:   &config.EnvironmentConfigArgoCd{Destination: config.ArgoCdDestination{Server: "prod-server"}},
	}
	_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		if err := dbHandler.DBWriteEnvironment(ctx, transaction, env, envConfig); err != nil {
			t.Fatalf("write env: %v", err)
		}
		seedCommittingCreateApp(ctx, t, dbHandler, transaction, "app-1", env, true)
		seedCommittingCreateApp(ctx, t, dbHandler, transaction, "app-2", env, true)
		return nil
	})
	_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		return baseRepo.Apply(ctx, transaction, &repository.CreateEnvironment{
			Environment:         env,
			Config:              envConfig,
			TransformerMetadata: repository.TransformerMetadata{AuthorName: "author", AuthorEmail: "email@example.com"},
		})
	})

	// First iteration: the push fails, so the cutoff must not advance and no commit timestamps are
	// written.
	sleepDuration := backoff.MakeSimpleBackoff(minSleep, maxSleep)
	if _, err := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, 2); err != nil {
		t.Fatalf("ProcessOneEvent (failing push): %v", err)
	}
	_ = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		cutoff, err := db.DBReadCutoff(dbHandler, ctx, transaction)
		if err != nil {
			t.Fatalf("read cutoff: %v", err)
		}
		if cutoff != nil {
			t.Errorf("expected cutoff to NOT advance after a failed push, got %v", *cutoff)
		}
		return nil
	})
	if got := countCommitTimestamps(ctx, t, dbHandler); got != 0 {
		t.Errorf("expected no commit-timestamp rows after a failed push, got %d", got)
	}

	// Recover: the next iteration reprocesses the whole batch cleanly.
	repo.fail = false
	sleepDuration.Reset()
	if _, err := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, 2); err != nil {
		t.Fatalf("ProcessOneEvent (recovered push): %v", err)
	}
	_ = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		cutoff, err := db.DBReadCutoff(dbHandler, ctx, transaction)
		if err != nil {
			t.Fatalf("read cutoff: %v", err)
		}
		if cutoff == nil || *cutoff != 2 {
			t.Errorf("expected cutoff 2 after recovery, got %v", cutoff)
		}
		return nil
	})
	if got := countCommitTimestamps(ctx, t, dbHandler); got != 2 {
		t.Errorf("expected 2 commit-timestamp rows after recovery, got %d", got)
	}
}

// TestProcessOneEventMixedSequence asserts the loop splits a mixed sequence correctly: a run of
// CreateApplicationVersion events is batched into one push, a following non-Create event is processed
// on its own, and a trailing Create is processed separately (Task 12).
func TestProcessOneEventMixedSequence(t *testing.T) {
	const minSleep = time.Nanosecond * 1
	const maxSleep = time.Nanosecond * 1000
	const env = types.EnvName("production")
	const env2 = types.EnvName("staging")
	ctx := context.Background()
	baseRepo, dbHandler, _ := SetupRepositoryTestWithDB(t, ctx)
	repo := &pushCountingRepo{Repository: baseRepo}

	envConfig := config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{Latest: true},
		ArgoCd:   &config.EnvironmentConfigArgoCd{Destination: config.ArgoCdDestination{Server: "prod-server"}},
	}
	env2Config := config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{Latest: true},
		ArgoCd:   &config.EnvironmentConfigArgoCd{Destination: config.ArgoCdDestination{Server: "staging-server"}},
	}
	// Sequence: Create(app-1), Create(app-2), CreateEnvironment(staging), Create(app-3).
	_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		if err := dbHandler.DBWriteEnvironment(ctx, transaction, env, envConfig); err != nil {
			t.Fatalf("write env: %v", err)
		}
		seedCommittingCreateApp(ctx, t, dbHandler, transaction, "app-1", env, true)
		seedCommittingCreateApp(ctx, t, dbHandler, transaction, "app-2", env, true)
		// a non-Create event between the runs of creates
		if err := dbHandler.DBWriteEnvironment(ctx, transaction, env2, env2Config); err != nil {
			t.Fatalf("write env2: %v", err)
		}
		//exhaustruct:ignore
		if err := dbHandler.DBWriteEslEventInternal(ctx, db.EvtCreateEnvironment, transaction, &repository.CreateEnvironment{Environment: env2, Config: env2Config}, db.ESLMetadata{AuthorName: "author", AuthorEmail: "email@example.com"}); err != nil {
			t.Fatalf("write CreateEnvironment esl event: %v", err)
		}
		seedCommittingCreateApp(ctx, t, dbHandler, transaction, "app-3", env, true)
		return nil
	})
	_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		return baseRepo.Apply(ctx, transaction, &repository.CreateEnvironment{
			Environment:         env,
			Config:              envConfig,
			TransformerMetadata: repository.TransformerMetadata{AuthorName: "author", AuthorEmail: "email@example.com"},
		})
	})

	// iteration -> expected total push count after it and expected cutoff after it
	steps := []struct {
		expectedPushes int
		expectedCutoff db.EslVersion
	}{
		{expectedPushes: 1, expectedCutoff: 2}, // batches the two creates
		{expectedPushes: 2, expectedCutoff: 3}, // the CreateEnvironment alone
		{expectedPushes: 3, expectedCutoff: 4}, // the trailing create
	}
	sleepDuration := backoff.MakeSimpleBackoff(minSleep, maxSleep)
	for i, step := range steps {
		if _, err := ProcessOneEvent(ctx, repo, dbHandler, nil, &sleepDuration, true, 10); err != nil {
			t.Fatalf("ProcessOneEvent iteration %d: %v", i, err)
		}
		if repo.pushCount != step.expectedPushes {
			t.Errorf("after iteration %d expected %d total pushes, got %d", i, step.expectedPushes, repo.pushCount)
		}
		_ = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
			cutoff, err := db.DBReadCutoff(dbHandler, ctx, transaction)
			if err != nil {
				t.Fatalf("read cutoff: %v", err)
			}
			if cutoff == nil || *cutoff != step.expectedCutoff {
				t.Errorf("after iteration %d expected cutoff %d, got %v", i, step.expectedCutoff, cutoff)
			}
			return nil
		})
	}
}
