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
	"testing"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
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

func convertToSet(list []types.ReleaseNumbers) map[TestStruct]bool {
	set := make(map[TestStruct]bool)
	for _, i := range list {
		set[TestStruct{Version: *i.Version, Revision: i.Revision}] = true
	}
	return set
}

func getTransformer(i int) (Transformer, error) {
	// transformerType := i % 5
	// switch transformerType {
	// case 0:
	// case 1:
	// case 2:
	return &CreateApplicationVersion{
		Application: "foo",
		Manifests: map[types.EnvName]string{
			"development": fmt.Sprintf("%d", i),
		},
		Version: uint64(i),
	}, nil
	// case 3:
	// 	return &ErrorTransformer{}, TransformerError
	// case 4:
	// 	return &InvalidJsonTransformer{}, InvalidJson
	// }
	// return &ErrorTransformer{}, TransformerError
}

func TestMeasureGitSyncStatus(t *testing.T) {
	tcs := []struct {
		Name             string
		SyncedFailedApps []db.EnvApp
		UnsyncedApps     []db.EnvApp
		ExpectedGauges   []Gauge
	}{
		{
			Name:             "No unsynced or sync failed apps",
			SyncedFailedApps: []db.EnvApp{},
			UnsyncedApps:     []db.EnvApp{},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 0, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 0, Tags: []string{}, Rate: 1},
			},
		},
		{
			Name: "Some sync failed apps",
			SyncedFailedApps: []db.EnvApp{
				{EnvName: "dev", AppName: "app"},
				{EnvName: "dev", AppName: "app2"},
			},
			UnsyncedApps: []db.EnvApp{
				{EnvName: "staging", AppName: "app"},
			},
			ExpectedGauges: []Gauge{
				{Name: "git_sync_unsynced", Value: 1, Tags: []string{}, Rate: 1},
				{Name: "git_sync_failed", Value: 2, Tags: []string{}, Rate: 1},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			var mockClient = &MockClient{}
			var client statsd.ClientInterface = mockClient

			ctx := testutilauth.MakeTestContext()
			repo := SetupRepositoryTestWithDB(t)
			ddMetrics = client
			dbHandler := repo.State().DBHandler

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.SyncedFailedApps, db.SYNC_FAILED)
				if err != nil {
					return err
				}

				err = dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, 0, tc.UnsyncedApps, db.UNSYNCED)
				if err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				t.Fatalf("failed to write sync events to db: %v", err)
			}

			err = MeasureGitSyncStatus(len(tc.UnsyncedApps), len(tc.SyncedFailedApps))
			if err != nil {
				t.Fatalf("failed to send git sync status metrics: %v", err)
			}

			cmpGauge := func(i, j Gauge) bool {
				if len(i.Tags) == 0 && len(j.Tags) == 0 {
					return i.Name > j.Name
				} else if len(i.Tags) != len(j.Tags) {
					return len(i.Tags) > len(j.Tags)
				} else {
					for tagIndex := range i.Tags {
						if i.Tags[tagIndex] != j.Tags[tagIndex] {
							return i.Tags[tagIndex] > j.Tags[tagIndex]
						}
					}
					return true
				}
			}
			if diff := cmp.Diff(tc.ExpectedGauges, mockClient.gauges, cmpopts.SortSlices(cmpGauge)); diff != "" {
				t.Errorf("gauges mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func SetupRepositoryBenchmark(t *testing.B) (Repository, *db.DBHandler) {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, fmt.Sprintf("%s_%d", t.Name(), t.N))
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}

	repoCfg := RepositoryConfig{
		ArgoCdGenerateFiles: true,
		AllowBracketMoves:   true,
	}

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	repoCfg.DBHandler = dbHandler

	repo, err := New(
		testutilauth.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dbHandler
}

type TestStruct struct {
	Version  uint64
	Revision uint64
}

func BenchmarkApplyQueue(t *testing.B) {
	t.StopTimer()
	repo, _ := SetupRepositoryBenchmark(t)
	ctx := testutilauth.MakeTestContext()
	dbHandler := repo.State().DBHandler

	repoInternal := repo.(*repository)
	// The worker go routine is now blocked. We can move some items into the queue now.
	results := make([]error, t.N)
	expectedResults := make([]error, t.N)

	expectedReleases := make(map[TestStruct]bool, t.N)

	err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
		if err != nil {
			return err
		}

		err = dbHandler.DBWriteEnvironment(ctx, transaction, "development", config.EnvironmentConfig{})
		if err != nil {
			return err
		}

		expectedResults[0] = nil
		results[0] = nil
		t.StartTimer()
		for i := 1; i < t.N; i++ {
			tf, expectedResult := getTransformer(i)
			_, _, _, err2 := repoInternal.ApplyTransformersInternal(ctx, transaction, tf)
			if err2 != nil {
				results[i] = err2.TransformerError
			} else {
				results[i] = nil
			}
			expectedResults[i] = expectedResult
			if expectedResult == nil {
				expectedReleases[TestStruct{Version: uint64(i), Revision: 0}] = true
			}
		}
		for i := 0; i < t.N; i++ {
			if diff := cmp.Diff(expectedResults[i], results[i], cmpopts.EquateErrors()); diff != "" {
				t.Errorf("result[%d] expected error \"%v\" but got \"%v\"", i, expectedResults[i], results[i])
			}
		}
		releases, _ := repo.State().GetAllApplicationReleases(ctx, transaction, "foo")

		if diff := cmp.Diff(expectedReleases, convertToSet(releases)); diff != "" {
			t.Fatalf("Output mismatch (-want +got): %s\n", diff)
		}

		return nil
	})
	if err != nil {
		t.Errorf("Error applying transformers: %v", err)
	}
}
