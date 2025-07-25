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

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/conversion"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
)

type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func createMigrationFolder(dbLocation string) (string, error) {
	loc := path.Join(dbLocation, "migrations")
	return loc, os.Mkdir(loc, os.ModePerm)
}

func TestMigrationScript(t *testing.T) {
	tcs := []struct {
		Name          string
		migrationFile string
		expectedData  []string
	}{
		{
			Name: "Simple migration",
			migrationFile: `
CREATE TABLE IF NOT EXISTS apps
(
    created TIMESTAMP,
    appName VARCHAR,
    stateChange VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY (appname)
);

INSERT INTO apps (created, appname, statechange, metadata)  VALUES ('2025-04-16 09:38:15', 'my-test-app', 'AppStateChangeMigrate', '{}');`,
			expectedData: []string{"my-test-app"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbDir := t.TempDir()
			_, dbConfig := SetupRepositoryTestWithDBMigrationPath(t, dbDir, false)
			dbConfig.MigrationsPath = dbConfig.MigrationsPath + "/migrations"
			loc, mkdirErr := createMigrationFolder(dbDir)
			if mkdirErr != nil {
				t.Fatalf("Error creating migrations folder. Error: %v", mkdirErr)
			}

			ts := time.Now().Unix()
			migrationFileNameAbsPath := path.Join(loc, strconv.FormatInt(ts, 10)+"_testing.up.sql")
			wErr := os.WriteFile(migrationFileNameAbsPath, []byte(tc.migrationFile), os.ModePerm)
			if wErr != nil {
				t.Fatalf("Error creating migration file. Error: %v", wErr)
			}

			migErr := RunDBMigrations(ctx, *dbConfig)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v", migErr)
			}

			db, err := Connect(ctx, *dbConfig)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}
			tx, err := db.BeginTransaction(ctx, false)
			if err != nil {
				t.Fatalf("Error creating transaction. Error: %v", err)
			}
			m, err := db.DBSelectAllApplications(ctx, tx)
			if err != nil {
				t.Fatalf("Error querying dabatabse. Error: %v", err)
			}
			err = tx.Commit()
			if err != nil {
				t.Fatalf("Error commiting transaction. Error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedData, m); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestCustomMigrationReleases(t *testing.T) {
	var getAllApps = /*GetAllAppsFun*/ func() (map[string]string, error) {
		result := map[string]string{
			"app1": "team1",
		}
		return result, nil
	}
	var writeAllReleases = /*writeAllReleases*/ func(ctx context.Context, transaction *sql.Tx, app string, dbHandler *DBHandler) error {
		releases := AllReleases{
			1: ReleaseWithManifest{
				Version:         666,
				UndeployVersion: false,
				SourceAuthor:    "auth1",
				SourceCommitId:  "commit1",
				SourceMessage:   "msg1",
				CreatedAt:       time.Time{},
				DisplayVersion:  "display1",
				Manifests: map[types.EnvName]string{
					"dev": "manifest1",
				},
			},
			2: ReleaseWithManifest{
				Version:         777,
				UndeployVersion: false,
				SourceAuthor:    "auth2",
				SourceCommitId:  "commit2",
				SourceMessage:   "msg2",
				CreatedAt:       time.Time{},
				DisplayVersion:  "display2",
				Manifests: map[types.EnvName]string{
					"dev": "manifest2",
				},
			},
		}
		for _, r := range releases {
			dbRelease := DBReleaseWithMetaData{
				Created: time.Now().UTC(),
				ReleaseNumbers: types.ReleaseNumbers{
					Version:  &r.Version,
					Revision: 0,
				},
				App: app,
				Manifests: DBReleaseManifests{
					Manifests: r.Manifests,
				},
				Metadata: DBReleaseMetaData{
					UndeployVersion: r.UndeployVersion,
					SourceAuthor:    r.SourceAuthor,
					SourceCommitId:  r.SourceCommitId,
					SourceMessage:   r.SourceMessage,
					DisplayVersion:  r.DisplayVersion,
				},
			}
			err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, dbRelease)
			if err != nil {
				return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
			}
		}
		return nil
	}
	tcs := []struct {
		Name             string
		expectedReleases []*DBReleaseWithMetaData
	}{
		{
			Name: "Simple migration",
			expectedReleases: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(666),
						Revision: 0,
					},
					App: "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev": "manifest1",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   "auth1",
						SourceCommitId: "commit1",
						SourceMessage:  "msg1",
						DisplayVersion: "display1",
					},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(777),
						Revision: 0,
					},
					App: "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"dev": "manifest2",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   "auth2",
						SourceCommitId: "commit2",
						SourceMessage:  "msg2",
						DisplayVersion: "display2",
					},
					Environments: []types.EnvName{"dev"},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dbHandler, _ := SetupRepositoryTestWithDB(t, true)
			err3 := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.RunCustomMigrationReleases(ctx, getAllApps, writeAllReleases)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				for i := range tc.expectedReleases {
					expectedRelease := tc.expectedReleases[i]

					release, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, expectedRelease.App, expectedRelease.ReleaseNumbers, true)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(expectedRelease, release, cmpopts.IgnoreFields(DBReleaseWithMetaData{}, "Created")); diff != "" {
						t.Errorf("error mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestCustomMigrationsApps(t *testing.T) {
	const appName = "my-app"
	const teamName = "my-team"
	tcs := []struct {
		Name            string
		expectedApps    []*DBAppWithMetaData
		expectedAllApps []string
		allAppsFunc     GetAllAppsFun
	}{
		{
			Name: "Simple migration",
			expectedApps: []*DBAppWithMetaData{
				{
					App:         appName,
					StateChange: AppStateChangeMigrate,
					Metadata: DBAppMetaData{
						Team: teamName,
					},
				},
			},
			expectedAllApps: []string{appName},
			allAppsFunc: func() (map[string]string, error) {
				result := map[string]string{
					appName: teamName,
				}
				return result, nil
			},
		},
		{
			Name:            "No apps still populate all_apps table",
			expectedApps:    []*DBAppWithMetaData{},
			expectedAllApps: []string{},
			allAppsFunc: func() (map[string]string, error) {
				result := map[string]string{}
				return result, nil
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dbHandler, _ := SetupRepositoryTestWithDB(t, true)
			err3 := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.RunAllCustomMigrationsForApps(ctx, tc.allAppsFunc)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}

				allApps, err2 := dbHandler.DBSelectAllApplications(ctx, transaction)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				if diff := cmp.Diff(tc.expectedAllApps, allApps); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				for i := range tc.expectedApps {
					expectedApp := tc.expectedApps[i]

					app, err := dbHandler.DBSelectApp(ctx, transaction, appName)
					if err != nil {
						return err
					}
					if diff := cmp.Diff(expectedApp, app); diff != "" {
						t.Errorf("error mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestMigrationCommitEvent(t *testing.T) {
	var writeAllCommitEvents = /*writeAllCommitEvents*/ func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error {
		return nil
	}
	tcs := []struct {
		Name           string
		expectedEvents []*event.DBEventGo
	}{
		{
			Name: "Test migration event",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dbHandler, _ := SetupRepositoryTestWithDB(t, true)
			err3 := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.RunCustomMigrationsEventSourcingLight(ctx)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}

				err2 = dbHandler.RunCustomMigrationsCommitEvents(ctx, writeAllCommitEvents)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				//Check for migration event
				contains, err := dbHandler.DBContainsMigrationCommitEvent(ctx, transaction)
				if err != nil {
					t.Errorf("could not get migration event: %v", err)

				}
				if !contains {
					t.Errorf("migration event was not created: %v", err)
				}
				return nil
			})
			if err3 != nil {
				t.Fatalf("expected no error, got %v", err3)
			}
		})
	}
}

func TestCommitEvents(t *testing.T) {

	tcs := []struct {
		Name       string
		commitHash string
		email      string
		event      event.DBEventGo
	}{
		{
			Name:       "Simple Deployment event",
			commitHash: "abcdefabcdef",
			email:      "test@email.com",

			event: event.DBEventGo{
				EventData: &event.Deployment{
					Environment:                 "production",
					Application:                 "test-app",
					SourceTrainUpstream:         nil,
					SourceTrainEnvironmentGroup: nil,
				},
				EventMetadata: event.Metadata{
					Uuid:      "00000000-0000-0000-0000-000000000001",
					EventType: "deployment",
				},
			},
		},
		{
			Name:       "Lock prevented deployment event",
			commitHash: "abcdefabcdef",
			email:      "test@email.com",
			event: event.DBEventGo{
				EventData: &event.LockPreventedDeployment{
					Environment: "production",
					Application: "test-app",
					LockMessage: "message",
					LockType:    "env",
				},
				EventMetadata: event.Metadata{
					Uuid:      "00000000-0000-0000-0000-000000000001",
					EventType: "lock-prevented-deployment",
				},
			},
		},
		{
			Name:       "Lock prevented deployment event",
			commitHash: "abcdefabcdef",
			email:      "test@email.com",
			event: event.DBEventGo{
				EventData: &event.NewRelease{
					Environments: map[string]struct{}{},
				},
				EventMetadata: event.Metadata{
					Uuid:      "00000000-0000-0000-0000-000000000001",
					EventType: "new-release",
				},
			},
		},
		{
			Name:       "New release event",
			commitHash: "abcdefabcdef",
			email:      "test@email.com",
			event: event.DBEventGo{
				EventData: &event.NewRelease{
					Environments: map[string]struct{}{"dev": {}},
				},
				EventMetadata: event.Metadata{
					Uuid:           "00000000-0000-0000-0000-000000000001",
					EventType:      "new-release",
					ReleaseVersion: 1,
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbDir := t.TempDir()

			dir, err := CreateMigrationsPath(2)
			if err != nil {
				t.Fatalf("setup error could not detect dir \n%v", err)
				return
			}
			_, dbConfig := SetupRepositoryTestWithDBMigrationPath(t, dbDir, false)
			dbConfig.MigrationsPath = dir

			migErr := RunDBMigrations(ctx, *dbConfig)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v", migErr)
			}

			db, err := Connect(ctx, *dbConfig)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}

			err = db.RunCustomMigrationsEventSourcingLight(ctx)
			if err != nil {
				t.Fatalf("Error running custom migrations for esl table. Error: %v", err)

			}
			err = db.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				if err != nil {
					t.Fatalf("Error creating transaction. Error: %v", err)
				}
				err := writeEventAux(ctx, db, tx, tc.commitHash, tc.event)
				if err != nil {
					t.Fatalf("Error writing event to DB. Error: %v", err)
				}

				m, err := db.DBSelectAllEventsForCommit(ctx, tx, tc.commitHash, 0, 100)
				if err != nil {
					t.Fatalf("Error querying dabatabse. Error: %v", err)
				}
				for _, currEvent := range m {
					e, err := event.UnMarshallEvent(event.EventType(tc.event.EventMetadata.EventType), currEvent.EventJson)

					if err != nil {
						t.Fatalf("Error obtaining event from DB. Error: %v", err)
					}

					if diff := cmp.Diff(e.EventData, tc.event.EventData); diff != "" {
						t.Errorf("response mismatch (-want, +got):\n%s", diff)
					}

					if diff := cmp.Diff(e.EventMetadata, tc.event.EventMetadata); diff != "" {
						t.Errorf("response mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Tranasction error")
			}
		})
	}
}

func writeEventAux(ctx context.Context, db *DBHandler, tx *sql.Tx, sourceCommitHash string, ev event.DBEventGo) error {
	jsonToInsert, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return db.WriteEvent(ctx, tx, 0, ev.EventMetadata.Uuid, event.EventType(ev.EventMetadata.EventType), sourceCommitHash, jsonToInsert)
}

func TestReadLockPreventedEvents(t *testing.T) {
	tcs := []struct {
		Name                 string
		Events               []*event.LockPreventedDeployment
		RequestTransformerID TransformerID
		ExpectedResults      []EventRow
	}{
		{
			Name: "One simple event",
			Events: []*event.LockPreventedDeployment{
				{
					Application: "app",
					Environment: "env",
					LockMessage: "test lock message",
					LockType:    "Application",
				},
			},
			RequestTransformerID: 0,
			ExpectedResults: []EventRow{
				{
					Uuid:       "00000000-0000-0000-0000-000000000000",
					CommitHash: "test",
					EventType:  "lock-prevented-deployment",
					EventJson:  `{"EventData":{"Application":"app","Environment":"env","LockMessage":"test lock message","LockType":"Application"},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"lock-prevented-deployment","ReleaseVersion":0}}`,
				},
			},
		},
		{
			Name: "Multiple events",
			Events: []*event.LockPreventedDeployment{
				{
					Application: "app",
					Environment: "env",
					LockMessage: "test lock message",
					LockType:    "Application",
				},
				{
					Application: "app2",
					Environment: "env2",
					LockMessage: "message2",
					LockType:    "Environment",
				},
			},
			RequestTransformerID: 0,
			ExpectedResults: []EventRow{
				{
					Uuid:       "00000000-0000-0000-0000-000000000000",
					CommitHash: "test",
					EventType:  "lock-prevented-deployment",
					EventJson:  `{"EventData":{"Application":"app","Environment":"env","LockMessage":"test lock message","LockType":"Application"},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"lock-prevented-deployment","ReleaseVersion":0}}`,
				},
				{
					Uuid:       "00000000-0000-0000-0000-000000000001",
					CommitHash: "test",
					EventType:  "lock-prevented-deployment",
					EventJson:  `{"EventData":{"Application":"app2","Environment":"env2","LockMessage":"message2","LockType":"Environment"},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000001","EventType":"lock-prevented-deployment","ReleaseVersion":0}}`,
				},
			},
		},
		{
			Name: "Wrong Transformer ID",
			Events: []*event.LockPreventedDeployment{
				{
					Application: "app",
					Environment: "env",
					LockMessage: "test lock message",
					LockType:    "Application",
				},
			},
			RequestTransformerID: 1,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbDir := t.TempDir()

			dir, err := CreateMigrationsPath(2)
			if err != nil {
				t.Fatalf("setup error could not detect dir \n%v", err)
				return
			}

			_, dbConfig := SetupRepositoryTestWithDBMigrationPath(t, dbDir, false)
			//dbConfig.MigrationsPath = dbConfig.MigrationsPath + "/migrations"
			dbConfig.MigrationsPath = dir

			migErr := RunDBMigrations(ctx, *dbConfig)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v", migErr)
			}

			db, err := Connect(ctx, *dbConfig)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}

			err = db.RunCustomMigrationsEventSourcingLight(ctx)
			if err != nil {
				t.Fatalf("Error running custom migrations for esl table. Error: %v", err)

			}
			err = db.WithTransactionR(ctx, 0, false, func(ctx context.Context, tx *sql.Tx) error {
				for i, event := range tc.Events {
					err = db.DBWriteLockPreventedDeploymentEvent(ctx, tx, 0, "00000000-0000-0000-0000-00000000000"+strconv.Itoa(i), "test", event)
					if err != nil {
						return err
					}
				}
				results, err := db.DBSelectAllLockPreventedEventsForTransformerID(ctx, tx, tc.RequestTransformerID)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.ExpectedResults, results, cmpopts.IgnoreFields(EventRow{}, "Timestamp")); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSqliteToPostgresQuery(t *testing.T) {
	tcs := []struct {
		Name          string
		inputQuery    string
		expectedQuery string
	}{
		{
			Name:          "select without parameter",
			inputQuery:    "select * from foo where true",
			expectedQuery: "select * from foo where true",
		},
		{
			Name:          "select with 1 parameter",
			inputQuery:    "select * from foo where bar = ?",
			expectedQuery: "select * from foo where bar = $1",
		},
		{
			Name:          "select with 3 parameter",
			inputQuery:    "SELECT * FROM foo WHERE bar = ? AND pow = ? OR this=?",
			expectedQuery: "SELECT * FROM foo WHERE bar = $1 AND pow = $2 OR this=$3",
		},
		{
			Name:          "insert with 3 parameter",
			inputQuery:    "INSERT INTO all_apps (version , created , json)  VALUES (?, ?, ?)",
			expectedQuery: "INSERT INTO all_apps (version , created , json)  VALUES ($1, $2, $3)",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			actualQuery := SqliteToPostgresQuery(tc.inputQuery)
			if diff := cmp.Diff(tc.expectedQuery, actualQuery); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	tcs := []struct {
		Name                string
		inputHandler        *DBHandler
		expectedEslTable    bool
		expectedOtherTables bool
	}{
		{
			Name:                "nil handler",
			inputHandler:        nil,
			expectedEslTable:    false,
			expectedOtherTables: false,
		},
		{
			Name: "esl only",
			inputHandler: &DBHandler{
				WriteEslOnly: true,
			},
			expectedEslTable:    true,
			expectedOtherTables: false,
		},
		{
			Name: "other tables",
			inputHandler: &DBHandler{
				WriteEslOnly: false,
			},
			expectedEslTable:    true,
			expectedOtherTables: true,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			actualEslTable := tc.inputHandler.ShouldUseEslTable()
			if diff := cmp.Diff(tc.expectedEslTable, actualEslTable); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
			actualOtherTables := tc.inputHandler.ShouldUseOtherTables()
			if diff := cmp.Diff(tc.expectedOtherTables, actualOtherTables); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func version(v int) *int64 {
	var result = int64(v)
	return &result
}
func uversion(v int) *uint64 {
	var result = uint64(v)
	return &result
}

func TestReadWriteDeployment(t *testing.T) {
	tcs := []struct {
		Name               string
		App                string
		Env                types.EnvName
		VersionToDeploy    *uint64
		ExpectedDeployment *Deployment
	}{
		{
			Name:            "with eslVersion != nil",
			App:             "app-a",
			Env:             "dev",
			VersionToDeploy: uversion(7),
			ExpectedDeployment: &Deployment{
				App: "app-a",
				Env: "dev",
				ReleaseNumbers: types.ReleaseNumbers{
					Revision: 0,
					Version:  uversion(7),
				},
				TransformerID: 0,
			},
		},
		{
			Name:            "with eslVersion == nil",
			App:             "app-b",
			Env:             "prod",
			VersionToDeploy: nil,
			ExpectedDeployment: &Deployment{
				App: "app-b",
				Env: "prod",
				ReleaseNumbers: types.ReleaseNumbers{
					Revision: 0,
					Version:  nil,
				},
				TransformerID: 0,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				deployment, err2 := dbHandler.DBSelectAnyDeployment(ctx, transaction)
				if err2 != nil {
					return err2
				}
				if deployment != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *deployment)
				}

				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, Deployment{
					App: tc.App,
					Env: tc.Env,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  tc.VersionToDeploy,
					},
					TransformerID: 0,
				})
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBSelectLatestDeployment(ctx, transaction, tc.App, tc.Env)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedDeployment, actual, cmpopts.IgnoreFields(Deployment{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadAllLatestDeploymentForApplication(t *testing.T) {
	tcs := []struct {
		Name                string
		AppName             string
		SetupDeployments    []*Deployment
		ExpectedDeployments map[types.EnvName]Deployment
	}{
		{
			Name:    "Select one deployment",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[types.EnvName]Deployment{
				"dev": {
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Select only latest deployment",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[types.EnvName]Deployment{
				"dev": {
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Select multiple deployments",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				{
					App: "app1",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(5),
					},
					TransformerID: 0,
				},
				{
					App: "app2",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(5),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[types.EnvName]Deployment{
				"dev": {
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				"staging": {
					App: "app1",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(5),
					},
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Select multiple deployments, with revisions",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 1,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				{
					App: "app1",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				{
					App: "app2",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[types.EnvName]Deployment{
				"dev": {
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 1,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
				"staging": {
					App: "app1",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(6),
					},
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Deployment with revisions, existing deployment gets replaced",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 5,
						Version:  uversion(10),
					},
					TransformerID: 0,
				},
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 12,
						Version:  uversion(10),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[types.EnvName]Deployment{
				"dev": {
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 12,
						Version:  uversion(10),
					},
					TransformerID: 0,
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				for _, deployment := range tc.SetupDeployments {
					err := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, *deployment)
					if err != nil {
						return err
					}
				}

				latestDeployments, err := dbHandler.DBSelectAllLatestDeploymentsForApplication(ctx, transaction, tc.AppName)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedDeployments, latestDeployments, cmpopts.IgnoreFields(Deployment{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadAllLatestDeployment(t *testing.T) {
	tcs := []struct {
		Name                string
		EnvName             types.EnvName
		SetupDeployments    []*Deployment
		ExpectedDeployments map[string]types.ReleaseNumbers
	}{
		{
			Name:    "Select one deployment",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App:            "app1",
					Env:            "dev",
					ReleaseNumbers: types.MakeReleaseNumberVersion(7),
					TransformerID:  0,
				},
			},
			ExpectedDeployments: map[string]types.ReleaseNumbers{
				"app1": types.MakeReleaseNumberVersion(7),
			},
		},
		{
			Name:    "Select latest deployment",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
				{
					App:            "app1",
					Env:            "dev",
					ReleaseNumbers: types.MakeReleaseNumberVersion(8),
					TransformerID:  0,
				},
			},
			ExpectedDeployments: map[string]types.ReleaseNumbers{
				"app1": types.MakeReleaseNumberVersion(8),
			},
		},
		{
			Name:    "Select multiple deployments",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App: "app1",
					Env: "dev",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(7),
					},
					TransformerID: 0,
				},
				{
					App:            "app2",
					Env:            "dev",
					ReleaseNumbers: types.MakeReleaseNumberVersion(8),
					TransformerID:  0,
				},
				{
					App: "app3",
					Env: "staging",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(8),
					},
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]types.ReleaseNumbers{
				"app1": types.MakeReleaseNumberVersion(7),
				"app2": types.MakeReleaseNumberVersion(8),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				for _, deployment := range tc.SetupDeployments {
					err := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, *deployment)
					if err != nil {
						return err
					}
				}

				latestDeployments, err := dbHandler.DBSelectAllLatestDeploymentsOnEnvironment(ctx, transaction, tc.EnvName)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedDeployments, latestDeployments, cmpopts.IgnoreFields(Deployment{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDeleteEnvironmentLock(t *testing.T) {
	tcs := []struct {
		Name          string
		Env           types.EnvName
		LockID        string
		Message       string
		AuthorName    string
		AuthorEmail   string
		CiLink        string
		ExpectedLocks []EnvironmentLock
	}{
		{
			Name:        "Write and delete",
			Env:         "dev",
			LockID:      "dev-lock",
			Message:     "My lock on dev",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			ExpectedLocks: []EnvironmentLock{
				{ //Sort DESC
					Env:        "dev",
					LockID:     "dev-lock",
					EslVersion: 2,
					Deleted:    true,
					Metadata: LockMetadata{
						Message:        "My lock on dev",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
				{
					Env:        "dev",
					LockID:     "dev-lock",
					EslVersion: 1,
					Deleted:    false,
					Metadata: LockMetadata{
						Message:        "My lock on dev",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				metadata := LockMetadata{
					Message:        tc.Message,
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
				}
				err := dbHandler.DBWriteEnvironmentLock(ctx, transaction, tc.LockID, tc.Env, metadata)
				if err != nil {
					return err
				}

				errDelete := dbHandler.DBDeleteEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if errDelete != nil {
					return err
				}

				actual, err := dbHandler.DBSelectEnvLockHistory(ctx, transaction, tc.Env, tc.LockID, 2)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(len(tc.ExpectedLocks), len(actual)); diff != "" {
					t.Fatalf("number of env locks mismatch (-want, +got):\n%s", diff)
				}

				if diff := cmp.Diff(&tc.ExpectedLocks, &actual, cmpopts.IgnoreFields(EnvironmentLock{}, "Created")); diff != "" {
					t.Fatalf("env locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestAllDeployments(t *testing.T) {
	const appName = "my-app"
	type data struct {
		EnvName types.EnvName
		Version uint64
	}
	tcs := []struct {
		Name     string
		AppName  string
		data     []data
		expected map[types.EnvName]types.ReleaseNumbers
	}{
		{
			Name:    "Simple Write",
			AppName: appName,
			data: []data{ //Updates in order
				{
					EnvName: "development",
					Version: 3,
				},
			},
			expected: map[types.EnvName]types.ReleaseNumbers{
				"development": types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name:    "Multiple Writes, no override",
			AppName: appName,
			data: []data{ //Updates in order
				{
					EnvName: "development",
					Version: 3,
				},
				{
					EnvName: "staging",
					Version: 2,
				},
			},
			expected: map[types.EnvName]types.ReleaseNumbers{
				"development": types.MakeReleaseNumberVersion(3),
				"staging":     types.MakeReleaseNumberVersion(2),
			},
		},
		{
			Name:    "Multiple Writes, override",
			AppName: appName,
			data: []data{ //Updates in order
				{
					EnvName: "development",
					Version: 3,
				},
				{
					EnvName: "staging",
					Version: 2,
				},
				{
					EnvName: "development",
					Version: 3,
				},
				{
					EnvName: "staging",
					Version: 3,
				},
			},
			expected: map[types.EnvName]types.ReleaseNumbers{
				"development": types.MakeReleaseNumberVersion(3),
				"staging":     types.MakeReleaseNumberVersion(3),
			},
		},
		{
			Name:    "Multiple Writes, override and downgrade",
			AppName: appName,
			data: []data{ //Updates in order
				{
					EnvName: "development",
					Version: 3,
				},
				{
					EnvName: "staging",
					Version: 2,
				},
				{
					EnvName: "development",
					Version: 1,
				},
				{
					EnvName: "staging",
					Version: 1,
				},
			},
			expected: map[types.EnvName]types.ReleaseNumbers{
				"development": types.MakeReleaseNumberVersion(1),
				"staging":     types.MakeReleaseNumberVersion(1),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}
				for _, d := range tc.data {
					deployment := Deployment{
						Created: time.Now(),
						App:     tc.AppName,
						Env:     d.EnvName,
						ReleaseNumbers: types.ReleaseNumbers{
							Revision: 0,
							Version:  &d.Version,
						},
						Metadata:      DeploymentMetadata{},
						TransformerID: 0,
					}
					err := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, deployment)
					if err != nil {
						t.Fatalf("Error updating all deployments: %v", err)
					}
				}
				result, err := dbHandler.DBSelectAllDeploymentsForApp(ctx, transaction, tc.AppName)
				if err != nil {
					t.Fatalf("Error reading from all deployments: %v", err)
				}
				if diff := cmp.Diff(tc.expected, result); diff != "" {
					t.Fatalf("mismatch result (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteEnvironmentLock(t *testing.T) {
	tcs := []struct {
		Name         string
		Env          types.EnvName
		LockID       string
		Message      string
		AuthorName   string
		AuthorEmail  string
		CiLink       string
		ExpectedLock *EnvironmentLock
	}{
		{
			Name:        "Simple environment lock",
			Env:         "dev",
			LockID:      "dev-lock",
			Message:     "My lock on dev",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			CiLink:      "www.test.com",
			ExpectedLock: &EnvironmentLock{
				Env:        "dev",
				LockID:     "dev-lock",
				EslVersion: 1,
				Deleted:    false,
				Metadata: LockMetadata{
					Message:        "My lock on dev",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
					CiLink:         "www.test.com",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				metadata := LockMetadata{
					Message:        tc.Message,
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
					CiLink:         tc.CiLink,
				}
				err := dbHandler.DBWriteEnvironmentLock(ctx, transaction, tc.LockID, tc.Env, metadata)
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBSelectEnvLockHistory(ctx, transaction, tc.Env, tc.LockID, 1)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(1, len(actual)); diff != "" {
					t.Fatalf("number of env locks mismatch (-want, +got):\n%s", diff)
				}
				target := actual[0]
				if diff := cmp.Diff(tc.ExpectedLock, &target, cmpopts.IgnoreFields(EnvironmentLock{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteApplicationLock(t *testing.T) {
	tcs := []struct {
		Name         string
		Env          types.EnvName
		LockID       string
		Message      string
		AppName      string
		AuthorName   string
		AuthorEmail  string
		CiLink       string
		ExpectedLock *ApplicationLockHistory
	}{
		{
			Name:        "Simple application lock",
			Env:         "dev",
			LockID:      "dev-app-lock",
			Message:     "My application lock on dev for my-app",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			AppName:     "my-app",
			CiLink:      "www.test.com",
			ExpectedLock: &ApplicationLockHistory{
				Env:     "dev",
				LockID:  "dev-app-lock",
				Deleted: false,
				App:     "my-app",
				Metadata: LockMetadata{
					Message:        "My application lock on dev for my-app",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
					CiLink:         "www.test.com",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectAppLock(ctx, transaction, tc.Env, tc.AppName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				err := dbHandler.DBWriteApplicationLock(ctx, transaction, tc.LockID, tc.Env, tc.AppName, LockMetadata{
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
					Message:        tc.Message,
					CiLink:         tc.CiLink,
				})
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBSelectAppLockHistory(ctx, transaction, tc.Env, tc.AppName, tc.LockID, 1)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(1, len(actual)); diff != "" {
					t.Fatalf("number of env locks mismatch (-want, +got):\n%s", diff)
				}
				target := actual[0]
				if diff := cmp.Diff(tc.ExpectedLock, &target, cmpopts.IgnoreFields(ApplicationLockHistory{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadAllActiveApplicationLock(t *testing.T) {

	type testLockInfo struct {
		Env         types.EnvName
		LockID      string
		Message     string
		AppName     string
		AuthorName  string
		AuthorEmail string
		CiLink      string
	}

	tcs := []struct {
		Name                string
		AppName             string
		SetupLocks          []testLockInfo
		DeleteLocks         []testLockInfo
		ExpectedActiveLocks []ApplicationLock
	}{
		{
			Name:    "Read one lock",
			AppName: "my-app",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{},
			ExpectedActiveLocks: []ApplicationLock{
				{
					Env:    "dev",
					LockID: "dev-app-lock",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
		{
			Name:    "Don't read deleted lock",
			AppName: "my-app",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: nil,
		},
		{
			Name:    "Only read not deleted locks",
			AppName: "my-app",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
				{
					Env:         "staging",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "staging",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: []ApplicationLock{
				{
					Env:    "dev",
					LockID: "dev-app-lock",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				for _, lockInfo := range tc.SetupLocks {
					err := dbHandler.DBWriteApplicationLock(ctx, transaction, lockInfo.LockID, lockInfo.Env, lockInfo.AppName, LockMetadata{
						CreatedByName:  lockInfo.AuthorName,
						CreatedByEmail: lockInfo.AuthorEmail,
						Message:        lockInfo.Message,
						CiLink:         lockInfo.CiLink,
					})
					if err != nil {
						return err
					}

				}

				for _, lockInfo := range tc.DeleteLocks {
					err := dbHandler.DBDeleteApplicationLock(ctx, transaction, lockInfo.Env, lockInfo.AppName, lockInfo.LockID)
					if err != nil {
						return err
					}
				}

				activeLocks, err := dbHandler.DBSelectAllActiveAppLocksForApp(ctx, transaction, tc.AppName)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedActiveLocks, activeLocks, cmpopts.IgnoreFields(ApplicationLock{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadAllActiveApplicationLockForApps(t *testing.T) {

	type testLockInfo struct {
		Env         types.EnvName
		LockID      string
		Message     string
		AppName     string
		AuthorName  string
		AuthorEmail string
		CiLink      string
	}

	tcs := []struct {
		Name                string
		AppNames            []string
		SetupLocks          []testLockInfo
		DeleteLocks         []testLockInfo
		ExpectedActiveLocks []ApplicationLock
	}{
		{
			Name: "Read one lock from one app",
			AppNames: []string{
				"my-app",
			},
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{},
			ExpectedActiveLocks: []ApplicationLock{
				{
					Env:    "dev",
					LockID: "dev-app-lock",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
		{
			Name: "Read two locks  from two apps",
			AppNames: []string{
				"my-app", "my-app-2",
			},
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
				{
					Env:         "dev",
					LockID:      "dev-app-lock-2",
					Message:     "My application lock on dev for my-app-2",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app-2",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{},
			ExpectedActiveLocks: []ApplicationLock{
				{
					Env:    "dev",
					LockID: "dev-app-lock",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:    "dev",
					LockID: "dev-app-lock-2",
					App:    "my-app-2",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app-2",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
		{
			Name: "Don't read deleted lock",
			AppNames: []string{
				"my-app",
			},
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: nil,
		},
		{
			Name: "Only read not deleted locks",
			AppNames: []string{
				"my-app", "my-app-2",
			},
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
				{
					Env:         "staging",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
				{
					Env:         "staging",
					LockID:      "dev-app-lock-staging",
					Message:     "My application lock on stagibg for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
				{
					Env:         "staging",
					LockID:      "dev-app-lock-staging-2",
					Message:     "My application lock on stagibg for my-app-2",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app-2",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "staging",
					LockID:      "dev-app-lock",
					Message:     "My application lock on dev for my-app",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					AppName:     "my-app",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: []ApplicationLock{
				{
					Env:    "dev",
					LockID: "dev-app-lock",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:    "staging",
					LockID: "dev-app-lock-staging",
					App:    "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on stagibg for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:    "staging",
					LockID: "dev-app-lock-staging-2",
					App:    "my-app-2",
					Metadata: LockMetadata{
						Message:        "My application lock on stagibg for my-app-2",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				for _, lockInfo := range tc.SetupLocks {
					err := dbHandler.DBWriteApplicationLock(ctx, transaction, lockInfo.LockID, lockInfo.Env, lockInfo.AppName, LockMetadata{
						CreatedByName:  lockInfo.AuthorName,
						CreatedByEmail: lockInfo.AuthorEmail,
						Message:        lockInfo.Message,
						CiLink:         lockInfo.CiLink,
					})
					if err != nil {
						return err
					}

				}

				for _, lockInfo := range tc.DeleteLocks {
					err := dbHandler.DBDeleteApplicationLock(ctx, transaction, lockInfo.Env, lockInfo.AppName, lockInfo.LockID)
					if err != nil {
						return err
					}
				}

				activeLocks, err := dbHandler.DBSelectAllActiveAppLocksForSliceApps(ctx, transaction, tc.AppNames)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedActiveLocks, activeLocks, cmpopts.IgnoreFields(ApplicationLock{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadAllActiveTeamLock(t *testing.T) {

	type testLockInfo struct {
		Env         types.EnvName
		LockID      string
		Message     string
		TeamName    string
		AuthorName  string
		AuthorEmail string
		CiLink      string
	}

	tcs := []struct {
		Name                string
		TeamName            string
		SetupLocks          []testLockInfo
		DeleteLocks         []testLockInfo
		ExpectedActiveLocks []TeamLock
	}{
		{
			Name:     "Read one lock",
			TeamName: "my-team",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-team-lock",
					Message:     "My team lock on dev for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{},
			ExpectedActiveLocks: []TeamLock{
				{
					Env:    "dev",
					LockID: "dev-team-lock",
					Team:   "my-team",
					Metadata: LockMetadata{
						Message:        "My team lock on dev for my-team",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
		{
			Name:     "Don't read deleted lock",
			TeamName: "my-team",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-team-lock",
					Message:     "My team lock on dev for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-team-lock",
					Message:     "My team lock on dev for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: []TeamLock{},
		},
		{
			Name:     "Only read active locks",
			TeamName: "my-team",
			SetupLocks: []testLockInfo{
				{
					Env:         "dev",
					LockID:      "dev-team-lock",
					Message:     "My team lock on dev for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
				{
					Env:         "staging",
					LockID:      "staging-team-lock",
					Message:     "My team lock on staging for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
			},
			DeleteLocks: []testLockInfo{
				{
					Env:         "staging",
					LockID:      "staging-team-lock",
					Message:     "My team lock on staging for my-team",
					AuthorName:  "myself",
					AuthorEmail: "myself@example.com",
					TeamName:    "my-team",
					CiLink:      "www.test.com",
				},
			},
			ExpectedActiveLocks: []TeamLock{
				{
					Env:    "dev",
					LockID: "dev-team-lock",
					Team:   "my-team",
					Metadata: LockMetadata{
						Message:        "My team lock on dev for my-team",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				for _, lockInfo := range tc.SetupLocks {
					err := dbHandler.DBWriteTeamLock(ctx, transaction, lockInfo.LockID, lockInfo.Env, lockInfo.TeamName, LockMetadata{
						CreatedByName:  lockInfo.AuthorName,
						CreatedByEmail: lockInfo.AuthorEmail,
						Message:        lockInfo.Message,
						CiLink:         lockInfo.CiLink,
					})
					if err != nil {
						return err
					}

				}

				for _, lockInfo := range tc.DeleteLocks {
					err := dbHandler.DBDeleteTeamLock(ctx, transaction, lockInfo.Env, lockInfo.TeamName, lockInfo.LockID)
					if err != nil {
						return err
					}
				}

				activeLocks, err := dbHandler.DBSelectAllActiveTeamLocksForTeam(ctx, transaction, tc.TeamName)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedActiveLocks, activeLocks, cmpopts.IgnoreFields(TeamLock{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDeleteApplicationLock(t *testing.T) {
	tcs := []struct {
		Name          string
		Env           types.EnvName
		LockID        string
		Message       string
		AppName       string
		AuthorName    string
		AuthorEmail   string
		ExpectedLocks []ApplicationLockHistory
		ExpectedError error
	}{
		{
			Name:        "Write and delete",
			Env:         "dev",
			LockID:      "dev-lock",
			AppName:     "myApp",
			Message:     "My lock on dev",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			ExpectedLocks: []ApplicationLockHistory{
				{ //Sort DESC
					Env:     "dev",
					App:     "myApp",
					LockID:  "dev-lock",
					Deleted: true,
					Metadata: LockMetadata{
						Message:        "My lock on dev",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
				{
					Env:     "dev",
					LockID:  "dev-lock",
					App:     "myApp",
					Deleted: false,
					Metadata: LockMetadata{
						Message:        "My lock on dev",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				err := dbHandler.DBWriteApplicationLock(ctx, transaction, tc.LockID, tc.Env, tc.AppName, LockMetadata{
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
					Message:        tc.Message,
					CiLink:         "",
				})
				if err != nil {
					return err
				}

				errDelete := dbHandler.DBDeleteApplicationLock(ctx, transaction, tc.Env, tc.AppName, tc.LockID)
				if errDelete != nil {
					return err
				}

				actual, err := dbHandler.DBSelectAppLockHistory(ctx, transaction, tc.Env, tc.AppName, tc.LockID, 2)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(len(tc.ExpectedLocks), len(actual)); diff != "" {
					t.Fatalf("number of env locks mismatch (-want, +got):\n%s", diff)
				}

				if diff := cmp.Diff(&tc.ExpectedLocks, &actual, cmpopts.IgnoreFields(ApplicationLockHistory{}, "Created")); diff != "" {
					t.Fatalf("app locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestQueueApplicationVersion(t *testing.T) {
	const envName = "dev"
	const appName = "deployment"

	tcs := []struct {
		Name                string
		Deployments         []QueuedDeployment
		ExpectedDeployments []QueuedDeployment
		Env                 types.EnvName
		AppName             string
		Version             *int64
	}{
		{
			Name: "Write and read",
			Deployments: []QueuedDeployment{
				{
					Env: envName,
					App: appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
			},
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
			},
		},
		{
			Name: "Write and read multiple",
			Deployments: []QueuedDeployment{
				{
					Env: envName,
					App: appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					Env: envName,
					App: appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
			},
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 2,
					Env:        envName,
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, deployments := range tc.Deployments {
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.ReleaseNumbers)
					if err != nil {
						return err
					}
				}

				actual, err := dbHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, envName, appName, 10)
				if err != nil {
					return err
				}
				if diff := cmp.Diff(tc.ExpectedDeployments, actual, cmpopts.IgnoreFields(QueuedDeployment{}, "Created")); diff != "" {
					t.Fatalf("env locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestQueueApplicationVersionDelete(t *testing.T) {
	const envName = "dev"
	const appName = "deployment"
	tcs := []struct {
		Name                string
		Env                 types.EnvName
		AppName             string
		ReleaseNumbers      *types.ReleaseNumbers
		ExpectedDeployments []QueuedDeployment
	}{
		{
			Name:    "Write and delete",
			Env:     envName,
			AppName: appName,
			ReleaseNumbers: &types.ReleaseNumbers{
				Revision: 0,
				Version:  uversion(1),
			},
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 2,
					Env:        envName,
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  nil,
					},
				},
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, tc.Env, tc.AppName, *tc.ReleaseNumbers)
				if err != nil {
					return err
				}

				err = dbHandler.DBDeleteDeploymentAttempt(ctx, transaction, tc.Env, tc.AppName)
				if err != nil {
					return err
				}
				actual, err := dbHandler.DBSelectDeploymentAttemptHistory(ctx, transaction, tc.Env, tc.AppName, len(tc.ExpectedDeployments))
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedDeployments, actual, cmpopts.IgnoreFields(QueuedDeployment{}, "Created")); diff != "" {
					t.Fatalf("env locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestAllQueuedApplicationVersionsOfApp(t *testing.T) {
	const appName = "foo"

	tcs := []struct {
		Name                string
		Deployments         []QueuedDeployment
		ExpectedDeployments []*QueuedDeployment
		App                 string
	}{
		{
			Name: "Read all queued versions on environment",
			App:  appName,
			Deployments: []QueuedDeployment{
				{
					Env: "dev",
					App: appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					Env: "staging",
					App: appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					EslVersion: 2,
					Env:        "staging",
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
				{
					Env: "dev",
					App: "bar",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
			},
			ExpectedDeployments: []*QueuedDeployment{
				{
					EslVersion: 1,
					Env:        "dev",
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					EslVersion: 2,
					Env:        "staging",
					App:        appName,
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, deployments := range tc.Deployments {
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.ReleaseNumbers)
					if err != nil {
						return err
					}
				}

				queuedDeployments, err := dbHandler.DBSelectLatestDeploymentAttemptOnAllEnvironments(ctx, transaction, tc.App)
				if err != nil {
					return err
				}

				slices.SortFunc(queuedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return types.Compare(d1.Env, d2.Env)
				})

				slices.SortFunc(tc.ExpectedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return types.Compare(d1.Env, d2.Env)
				})

				if diff := cmp.Diff(tc.ExpectedDeployments, queuedDeployments, cmpopts.IgnoreFields(QueuedDeployment{}, "Created")); diff != "" {
					t.Fatalf("env locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestAllQueuedApplicationVersionsOnEnvironment(t *testing.T) {
	const envName = "dev"

	tcs := []struct {
		Name                string
		Deployments         []QueuedDeployment
		ExpectedDeployments []*QueuedDeployment
		Env                 types.EnvName
	}{
		{
			Name: "Read all queued versions on environment",
			Env:  envName,
			Deployments: []QueuedDeployment{
				{
					Env: envName,
					App: "foo",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					Env: envName,
					App: "bar",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					EslVersion: 2,
					Env:        envName,
					App:        "bar",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
				{
					Env: "fakeEnv",
					App: "foo",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
			},
			ExpectedDeployments: []*QueuedDeployment{
				{
					EslVersion: 1,
					Env:        envName,
					App:        "foo",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(0),
					},
				},
				{
					EslVersion: 2,
					Env:        envName,
					App:        "bar",
					ReleaseNumbers: types.ReleaseNumbers{
						Revision: 0,
						Version:  uversion(1),
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, deployments := range tc.Deployments {
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.ReleaseNumbers)
					if err != nil {
						return err
					}
				}

				queuedDeployments, err := dbHandler.DBSelectLatestDeploymentAttemptOfAllApps(ctx, transaction, tc.Env)
				if err != nil {
					return err
				}

				slices.SortFunc(queuedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return strings.Compare(d1.App, d2.App)
				})

				slices.SortFunc(tc.ExpectedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return strings.Compare(d1.App, d2.App)
				})

				if diff := cmp.Diff(tc.ExpectedDeployments, queuedDeployments, cmpopts.IgnoreFields(QueuedDeployment{}, "Created")); diff != "" {
					t.Fatalf("env locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteTeamLock(t *testing.T) {
	tcs := []struct {
		Name         string
		Env          types.EnvName
		LockID       string
		Message      string
		TeamName     string
		AuthorName   string
		AuthorEmail  string
		CiLink       string
		ExpectedLock *TeamLockHistory
	}{
		{
			Name:        "Simple application lock",
			Env:         "dev",
			LockID:      "dev-team-lock",
			Message:     "My team lock on dev for my-team",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			TeamName:    "my-team",
			CiLink:      "www.test.com",
			ExpectedLock: &TeamLockHistory{
				Env:    "dev",
				LockID: "dev-team-lock",
				Team:   "my-team",
				Metadata: LockMetadata{
					Message:        "My team lock on dev for my-team",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
					CiLink:         "www.test.com",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectTeamLock(ctx, transaction, tc.Env, tc.TeamName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				err := dbHandler.DBWriteTeamLock(ctx, transaction, tc.LockID, tc.Env, tc.TeamName, LockMetadata{
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
					Message:        tc.Message,
					CiLink:         tc.CiLink,
				})
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBSelectTeamLockHistory(ctx, transaction, tc.Env, tc.TeamName, tc.LockID, 1)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(1, len(actual)); diff != "" {
					t.Fatalf("number of team locks mismatch (-want, +got):\n%s", diff)
				}
				target := actual[0]
				if diff := cmp.Diff(tc.ExpectedLock, &target, cmpopts.IgnoreFields(TeamLockHistory{}, "Created")); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDeleteTeamLock(t *testing.T) {
	tcs := []struct {
		Name          string
		Env           types.EnvName
		LockID        string
		Message       string
		TeamName      string
		AuthorName    string
		AuthorEmail   string
		ExpectedLocks []TeamLockHistory
		ExpectedError error
	}{
		{
			Name:        "Write and delete",
			Env:         "dev",
			LockID:      "dev-lock",
			TeamName:    "my-team",
			Message:     "My lock on dev for my-team",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			ExpectedLocks: []TeamLockHistory{
				{ //Sort DESC
					Env:    "dev",
					Team:   "my-team",
					LockID: "dev-lock",
					Metadata: LockMetadata{
						Message:        "My lock on dev for my-team",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
					Deleted: true,
				},
				{
					Env:    "dev",
					LockID: "dev-lock",
					Team:   "my-team",
					Metadata: LockMetadata{
						Message:        "My lock on dev for my-team",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectTeamLock(ctx, transaction, tc.Env, tc.TeamName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return fmt.Errorf("expected no eslVersion, but got %v", *envLock)
				}
				err := dbHandler.DBWriteTeamLock(ctx, transaction, tc.LockID, tc.Env, tc.TeamName, LockMetadata{
					CreatedByName:  tc.AuthorName,
					CreatedByEmail: tc.AuthorEmail,
					Message:        tc.Message,
					CiLink:         "",
				})
				if err != nil {
					return err
				}

				errDelete := dbHandler.DBDeleteTeamLock(ctx, transaction, tc.Env, tc.TeamName, tc.LockID)
				if errDelete != nil {
					return err
				}

				actual, err := dbHandler.DBSelectTeamLockHistory(ctx, transaction, tc.Env, tc.TeamName, tc.LockID, 2)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(len(tc.ExpectedLocks), len(actual)); diff != "" {
					t.Fatalf("number of team locks mismatch (-want, +got):\n%s", diff)
				}

				if diff := cmp.Diff(&tc.ExpectedLocks, &actual, cmpopts.IgnoreFields(TeamLockHistory{}, "Created")); diff != "" {
					t.Fatalf("team locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDeleteRelease(t *testing.T) {
	tcs := []struct {
		Name     string
		toInsert DBReleaseWithMetaData
	}{
		{
			Name: "Delete Release from database",
			toInsert: DBReleaseWithMetaData{
				Created:        time.Now(),
				ReleaseNumbers: types.MakeReleaseNumberVersion(1),
				App:            "app",
				Manifests: DBReleaseManifests{
					Manifests: map[types.EnvName]string{"development": "development"},
				},
				Metadata: DBReleaseMetaData{
					SourceAuthor:   "me",
					SourceCommitId: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					SourceMessage:  "message",
					DisplayVersion: "1.0.0",
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, tc.toInsert)
				if err2 != nil {
					return err2
				}

				errDelete := dbHandler.DBDeleteFromReleases(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumbers)
				if errDelete != nil {
					t.Fatalf("error: %v", errDelete)
				}

				allReleases, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, tc.toInsert.App)
				if err != nil {
					return err
				}
				if len(allReleases) != 0 {
					t.Fatalf("number of team locks mismatch (-want, +got):\n%d", len(allReleases))
				}

				latestRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumbers, true)
				if err != nil {
					return err
				}
				if latestRelease != nil {
					t.Fatalf("Release has not beed deleted\n")
				}

				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteEnvironment(t *testing.T) {
	type EnvAndConfig struct {
		EnvironmentName   types.EnvName
		EnvironmentConfig config.EnvironmentConfig
		Applications      []string
	}
	type TestCase struct {
		Name          string
		EnvsToWrite   []EnvAndConfig
		EnvToQuery    types.EnvName
		ExpectedEntry *DBEnvironment
	}

	testCases := []TestCase{
		{
			Name: "write one environment and read it",
			EnvsToWrite: []EnvAndConfig{
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
			},
			EnvToQuery: "development",
			ExpectedEntry: &DBEnvironment{
				Name:         "development",
				Config:       testutil.MakeEnvConfigLatest(nil),
				Applications: []string{"app1", "app2", "app3"},
			},
		},
		{
			Name: "write one environment and read it, but this time with a more elaborate config",
			EnvsToWrite: []EnvAndConfig{
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")), // "elaborate config" being the env group
					Applications:      []string{"app1"},
				},
			},
			EnvToQuery: "development",
			ExpectedEntry: &DBEnvironment{
				Name:         "development",
				Config:       testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")),
				Applications: []string{"app1"},
			},
		},
		{
			Name: "write one environment two times and read it",
			EnvsToWrite: []EnvAndConfig{
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil), // without group
				},
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")), // with group
				},
			},
			EnvToQuery: "development",
			ExpectedEntry: &DBEnvironment{
				Name:   "development",
				Config: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")),
			},
		},
		{
			Name: "write one environment three times and read it",
			EnvsToWrite: []EnvAndConfig{
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil), // without group
				},
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")), // with group
				},
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-development-group")), // with group
				},
			},
			EnvToQuery: "development",
			ExpectedEntry: &DBEnvironment{
				Name:   "development",
				Config: testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-development-group")),
			},
		},
		{
			Name: "write multiple environments and read one of them",
			EnvsToWrite: []EnvAndConfig{
				EnvAndConfig{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
				},
				EnvAndConfig{
					EnvironmentName:   "staging",
					EnvironmentConfig: testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			EnvToQuery: "staging",
			ExpectedEntry: &DBEnvironment{
				Name:   "staging",
				Config: testutil.MakeEnvConfigUpstream("development", nil),
			},
		},
		{
			Name:          "don't write any environments and query something",
			EnvToQuery:    "development",
			ExpectedEntry: nil,
		},
		{
			Name: "write some environment, query something else",
			EnvsToWrite: []EnvAndConfig{
				EnvAndConfig{
					EnvironmentName:   "staging",
					EnvironmentConfig: testutil.MakeEnvConfigUpstream("development", nil),
				},
			},
			EnvToQuery: "development",
		},
		{
			Name: "write one environment and read it, test that it orders applications",
			EnvsToWrite: []EnvAndConfig{
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"zapp", "app1", "capp"},
				},
			},
			EnvToQuery: "development",
			ExpectedEntry: &DBEnvironment{
				Name:         "development",
				Config:       testutil.MakeEnvConfigLatest(nil),
				Applications: []string{"app1", "capp", "zapp"},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			for _, envToWrite := range tc.EnvsToWrite {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, envToWrite.EnvironmentName, envToWrite.EnvironmentConfig, envToWrite.Applications)
					if err != nil {
						return fmt.Errorf("error while writing environment, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing environment %s to the database, error: %v", envToWrite.EnvironmentName, err)
				}
			}

			envEntry, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*DBEnvironment, error) {
				envEntry, err := dbHandler.DBSelectEnvironment(ctx, transaction, tc.EnvToQuery)
				if err != nil {
					return nil, fmt.Errorf("error while selecting environment entry, error: %w", err)
				}
				return envEntry, nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for selecting the target environment, error: %v", err)
			}
			if diff := cmp.Diff(envEntry, tc.ExpectedEntry, cmpopts.IgnoreFields(DBEnvironment{}, "Created")); diff != "" {
				t.Fatalf("the received environment entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s", tc.ExpectedEntry, envEntry, diff)
			}
		})
	}
}

func TestReadEnvironmentBatch(t *testing.T) {
	type EnvAndConfig struct {
		EnvironmentName   types.EnvName
		EnvironmentConfig config.EnvironmentConfig
		Applications      []string
	}
	type TestCase struct {
		Name         string
		EnvsToWrite  []EnvAndConfig
		EnvsToQuery  []types.EnvName
		ExpectedEnvs *[]DBEnvironment
	}

	testCases := []TestCase{
		{
			Name: "read batch of environments",
			EnvsToWrite: []EnvAndConfig{
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
				{
					EnvironmentName:   "staging",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
				{
					EnvironmentName:   "production",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
			},
			EnvsToQuery: []types.EnvName{"development", "staging"},
			ExpectedEnvs: &[]DBEnvironment{
				{
					Name:         "development",
					Config:       testutil.MakeEnvConfigLatest(nil),
					Applications: []string{"app1", "app2", "app3"},
				},
				{
					Name:         "staging",
					Config:       testutil.MakeEnvConfigLatest(nil),
					Applications: []string{"app1", "app2", "app3"},
				},
			},
		},
		{
			Name: "read only latest esl version of environments",
			EnvsToWrite: []EnvAndConfig{
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
				{
					EnvironmentName:   "staging",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2", "app3"},
				},
				{
					EnvironmentName:   "development",
					EnvironmentConfig: testutil.MakeEnvConfigLatest(nil),
					Applications:      []string{"app1", "app2"},
				},
			},
			EnvsToQuery: []types.EnvName{"development", "staging"},
			ExpectedEnvs: &[]DBEnvironment{
				{
					Name:         "development",
					Config:       testutil.MakeEnvConfigLatest(nil),
					Applications: []string{"app1", "app2"},
				},
				{
					Name:         "staging",
					Config:       testutil.MakeEnvConfigLatest(nil),
					Applications: []string{"app1", "app2", "app3"},
				},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			for _, envToWrite := range tc.EnvsToWrite {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, envToWrite.EnvironmentName, envToWrite.EnvironmentConfig, envToWrite.Applications)
					if err != nil {
						return fmt.Errorf("error while writing environment, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing environment %s to the database, error: %v", envToWrite.EnvironmentName, err)
				}
			}

			environments, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*[]DBEnvironment, error) {
				enviroments, err := dbHandler.DBSelectEnvironmentsBatch(ctx, transaction, tc.EnvsToQuery)
				if err != nil {
					return nil, fmt.Errorf("error while selecting environment batch, error: %w", err)
				}
				return enviroments, nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for selecting the target environment, error: %v", err)
			}
			if diff := cmp.Diff(environments, tc.ExpectedEnvs, cmpopts.IgnoreFields(DBEnvironment{}, "Created")); diff != "" {
				t.Fatalf("the received environment entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s", tc.ExpectedEnvs, environments, diff)
			}
		})
	}
}

func TestReadWriteEslEvent(t *testing.T) {
	const envName = "dev"
	const appName = "my-app"
	const lockId = "ui-v2-ke1up"
	const message = "test"
	const authorName = "testauthor"
	const authorEmail = "testemail@example.com"
	const evtType = EvtCreateApplicationVersion
	expectedJson, _ := json.Marshal(map[string]interface{}{
		"env":     envName,
		"app":     appName,
		"lockId":  lockId,
		"message": message,
		"metadata": map[types.EnvName]string{
			"authorEmail": authorEmail,
			"authorName":  authorName,
		},
	})
	expectedJsonWithoutMetadata, _ := json.Marshal(map[string]interface{}{
		"env":     envName,
		"app":     appName,
		"lockId":  lockId,
		"message": message,
		"metadata": map[types.EnvName]string{
			"authorEmail": "",
			"authorName":  "",
		},
	})

	tcs := []struct {
		Name          string
		EventType     EventType
		EventData     *map[types.EnvName]string
		EventMetadata ESLMetadata
		ExpectedEsl   EslEventRow
	}{
		{
			Name:      "Write and read",
			EventType: evtType,
			EventData: &map[types.EnvName]string{
				"env":     envName,
				"app":     appName,
				"lockId":  lockId,
				"message": message,
			},
			EventMetadata: ESLMetadata{
				AuthorName:  authorName,
				AuthorEmail: authorEmail,
			},
			ExpectedEsl: EslEventRow{
				EventType: evtType,
				EventJson: string(expectedJson),
			},
		},
		{
			Name:      "Write and read with nil metadata",
			EventType: evtType,
			EventData: &map[types.EnvName]string{
				"env":     envName,
				"app":     appName,
				"lockId":  lockId,
				"message": message,
			},
			ExpectedEsl: EslEventRow{
				EventType: evtType,
				EventJson: string(expectedJsonWithoutMetadata),
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteEslEventInternal(ctx, tc.EventType, transaction, tc.EventData, tc.EventMetadata)
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBReadEslEventInternal(ctx, transaction, true)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedEsl.EventType, actual.EventType); diff != "" {
					t.Fatalf("event type mismatch (-want, +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.ExpectedEsl.EventJson, actual.EventJson); diff != "" {
					t.Fatalf("event json mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteFailedEslEvent(t *testing.T) {
	tcs := []struct {
		Name   string
		Events []EslFailedEventRow
		Limit  int
	}{
		{
			Name: "Write and read once",
			Events: []EslFailedEventRow{
				{
					EventType:             EvtCreateApplicationVersion,
					EventJson:             string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            1,
					Reason:                "",
					TransformerEslVersion: 0,
				},
			},
			Limit: 1,
		},
		{
			Name: "Write and read multiple",
			Events: []EslFailedEventRow{
				{
					EventType:             EvtCreateApplicationVersion,
					EventJson:             string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            1,
					Reason:                "",
					TransformerEslVersion: 0,
				},
				{
					EventType:             EvtCreateEnvironmentApplicationLock,
					EventJson:             string(`{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            2,
					Reason:                "unexpected error",
					TransformerEslVersion: 0,
				},
				{
					EventType:             EvtCreateEnvironment,
					EventJson:             string(`{"env":"dev3","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            3,
					Reason:                "unknown error",
					TransformerEslVersion: 0,
				},
			},
			Limit: 3,
		},
		{
			Name: "More than limit",
			Events: []EslFailedEventRow{
				{
					EventType:             EvtCreateApplicationVersion,
					EventJson:             string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            1,
					Reason:                "failed to create app version",
					TransformerEslVersion: 0,
				},
				{
					EventType:             EvtCreateEnvironmentGroupLock,
					EventJson:             string(`{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            2,
					Reason:                "",
					TransformerEslVersion: 0,
				},
				{
					EventType:             EvtCreateEnvironment,
					EventJson:             string(`{"env":"dev3","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            3,
					Reason:                "unexpected error",
					TransformerEslVersion: 0,
				},
			},
			Limit: 2,
		},
		{
			Name: "Less than limit",
			Events: []EslFailedEventRow{
				{
					EventType:             EvtCreateApplicationVersion,
					EventJson:             string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:               time.Now(),
					EslVersion:            1,
					Reason:                "",
					TransformerEslVersion: 0,
				},
			},
			Limit: 3,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				for _, event := range tc.Events {
					err := dbHandler.DBWriteFailedEslEvent(ctx, transaction, "event_sourcing_light_failed_history", &event)
					if err != nil {
						return err
					}
				}

				actualEvents, err := dbHandler.DBReadLastFailedEslEvents(ctx, transaction, 25, 0)
				if err != nil {
					return err
				}

				if len(actualEvents) > tc.Limit {
					t.Fatalf("expected %d events, got %d", tc.Limit, len(actualEvents))
				}

				for i, actualEvent := range actualEvents {
					reverse_index := len(tc.Events) - 1 - i // The order of the results should be descending
					if diff := cmp.Diff(tc.Events[reverse_index].EslVersion, actualEvent.EslVersion); diff != "" {
						t.Fatalf("event id mismatch (-want, +got):\n%s", diff)
					}
					if diff := cmp.Diff(tc.Events[reverse_index].EventType, actualEvent.EventType); diff != "" {
						t.Fatalf("event type mismatch (-want, +got):\n%s", diff)
					}
					if diff := cmp.Diff(tc.Events[reverse_index].EventJson, actualEvent.EventJson); diff != "" {
						t.Fatalf("event json mismatch (-want, +got):\n%s", diff)
					}
					if diff := cmp.Diff(tc.Events[reverse_index].Reason, actualEvent.Reason); diff != "" {
						t.Fatalf("event reason mismatch (-want, +got):\n%s", diff)
					}
					if diff := cmp.Diff(tc.Events[reverse_index].TransformerEslVersion, actualEvent.TransformerEslVersion); diff != "" {
						t.Fatalf("event transformer mismatch (-want, +got):\n%s", diff)
					}
				}

				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestReadWriteAllEnvironments(t *testing.T) {
	type TestCase struct {
		Name           string
		AllEnvsToWrite []types.EnvName
		ExpectedEntry  []types.EnvName
	}
	testCases := []TestCase{
		{
			Name:           "create entry with one environment entry only",
			AllEnvsToWrite: []types.EnvName{"development"},
			ExpectedEntry:  []types.EnvName{"development"},
		},
		{
			Name:           "create entries with increasing length",
			AllEnvsToWrite: []types.EnvName{"development", "production", "staging"},
			ExpectedEntry:  []types.EnvName{"development", "production", "staging"},
		},
		{
			Name:           "ensure that environments are sorted",
			AllEnvsToWrite: []types.EnvName{"staging", "development", "production"},
			ExpectedEntry:  []types.EnvName{"development", "production", "staging"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			for _, envName := range tc.AllEnvsToWrite {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, envName, config.EnvironmentConfig{}, []string{})
					if err != nil {
						return fmt.Errorf("error while writing environment, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing environments %v to the database, error: %v", tc.AllEnvsToWrite, err)
				}
			}

			allEnvsEntry, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*[]types.EnvName, error) {
				allEnvsEntry, err := dbHandler.DBSelectAllEnvironments(ctx, transaction)
				if err != nil {
					return nil, fmt.Errorf("error while selecting environment entry, error: %w", err)
				}
				return &allEnvsEntry, nil
			})

			if err != nil {
				t.Fatalf("error while running the transaction for selecting the target all environment, error: %v", err)
			}
			if diff := cmp.Diff(*allEnvsEntry, tc.ExpectedEntry); diff != "" {
				t.Fatalf("the received entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s", tc.ExpectedEntry, allEnvsEntry, diff)
			}
		})
	}
}

func TestReadWriteAllApplications(t *testing.T) {
	type TestCase struct {
		Name           string
		AllAppsToWrite []string
		ExpectedEntry  []string
	}

	testCases := []TestCase{
		{
			Name:           "test that app are ordered",
			AllAppsToWrite: []string{"my_app", "ze_app", "the_app"},
			ExpectedEntry:  []string{"my_app", "the_app", "ze_app"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, appname := range tc.AllAppsToWrite {
					err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, appname, AppStateChangeCreate, DBAppMetaData{})
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing all applications %v to the database, error: %v", tc.AllAppsToWrite, err)
			}

			allAppsEntry, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*[]string, error) {
				allAppsEntry, err := dbHandler.DBSelectAllApplications(ctx, transaction)
				if err != nil {
					return nil, fmt.Errorf("error while selecting application entry, error: %w", err)
				}
				return &allAppsEntry, nil
			})

			if err != nil {
				t.Fatalf("error while running the transaction for selecting the target all applications, error: %v", err)
			}
			if diff := cmp.Diff(*allAppsEntry, tc.ExpectedEntry); diff != "" {
				t.Fatalf("the received entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s", tc.ExpectedEntry, *allAppsEntry, diff)
			}
		})
	}
}

func TestReadReleasesByApp(t *testing.T) {

	tcs := []struct {
		Name                 string
		Releases             []DBReleaseWithMetaData
		RetrievePrepublishes bool
		AppName              string
		Expected             []*DBReleaseWithMetaData
		ExpectedErr          error
	}{
		{
			Name: "Retrieve one release",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
					Environments:   []types.EnvName{"dev"},
				},
			},
		},
		{
			Name: "Retrieved release has ordered environments",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manfest2", "production": "manfest2"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manfest2", "production": "manfest2"}},
					Environments:   []types.EnvName{"dev", "production", "staging"},
				},
			},
		},
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest4"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
					Environments:   []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
					Environments:   []types.EnvName{"dev"},
				},
			},
		},
		{
			Name: "Retrieve no releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1`",
			Expected: nil,
		},
		{
			Name: "Different Releases with different eslVersions",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
					Environments: []types.EnvName{"dev"},
				},
			},
		},
		{
			Name: "Prepublish Release should not be retrieved",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
					Metadata:  DBReleaseMetaData{IsPrepublish: true},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: false,
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
					Environments: []types.EnvName{"dev"},
				},
			},
		},
		{
			Name: "Prepublish Release should be retrieved",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
					Metadata:  DBReleaseMetaData{IsPrepublish: true},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: true,
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(3),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest3"}},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest2"}},
					Metadata:     DBReleaseMetaData{IsPrepublish: true},
					Environments: []types.EnvName{"dev"},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:          "app1",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
					Environments: []types.EnvName{"dev"},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				releases, err := dbHandler.DBSelectReleasesByAppLatestEslVersion(ctx, transaction, tc.AppName, !tc.RetrievePrepublishes)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, releases, cmpopts.IgnoreFields(DBReleaseWithMetaData{}, "Created")); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestReadReleasesByVersion(t *testing.T) {

	tcs := []struct {
		Name                 string
		Releases             []DBReleaseWithMetaData
		RetrievePrepublishes bool
		AppName              string
		Versions             []uint64
		Expected             []*DBReleaseWithMetaData
	}{
		{
			Name: "Retrieve one release, no manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Environments:   []types.EnvName{"dev"},
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
			},
		},
		{
			Name: "Retrieve no releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{},
			Expected: []*DBReleaseWithMetaData{},
		},
		{
			Name: "Retrieve one of two releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(11),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{11},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(11),
						Revision: 0,
					},
					App:          "app1",
					Environments: []types.EnvName{"dev"},
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
			},
		},
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(11),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10, 11},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Environments:   []types.EnvName{"dev"},
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(11),
						Revision: 0,
					},
					App:          "app1",
					Environments: []types.EnvName{"dev"},
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
			},
		},
		{
			Name: "Retrieve latest esl version only",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manifest2"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Environments:   []types.EnvName{"dev", "staging"},
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				releases, err := dbHandler.DBSelectReleasesByVersions(ctx, transaction, tc.AppName, tc.Versions, !tc.RetrievePrepublishes)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, releases, cmpopts.IgnoreFields(DBReleaseWithMetaData{}, "Created")); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestReadAllReleasesOfAllApps(t *testing.T) {

	tcs := []struct {
		Name     string
		Releases []DBReleaseWithMetaData
		Expected map[string][]types.ReleaseNumbers
	}{
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(1),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(2),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(1),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(2),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			Expected: map[string][]types.ReleaseNumbers{
				"app1": {types.MakeReleaseNumberVersion(2), types.MakeReleaseNumberVersion(1)},
				"app2": {types.MakeReleaseNumberVersion(2), types.MakeReleaseNumberVersion(1)},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			allReleases := map[string][]int64{}
			for _, release := range tc.Releases {
				if _, ok := allReleases[release.App]; !ok {
					allReleases[release.App] = []int64{int64(*release.ReleaseNumbers.Version)}
				} else {
					allReleases[release.App] = append(allReleases[release.App], int64(*release.ReleaseNumbers.Version))
				}
			}

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return err
					}
				}
				releases, err := dbHandler.DBSelectAllReleasesOfAllApps(ctx, transaction)

				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, releases); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestReadAllManifestsAllReleases(t *testing.T) {
	tcs := []struct {
		Name     string
		Releases []DBReleaseWithMetaData
		Expected AppVersionEnvironments
	}{
		{
			Name: "Retrieve no manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{}},
				},
			},
			Expected: AppVersionEnvironments{
				"app1": {"1.0": {}},
			},
		},
		{
			Name: "Retrieve all manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manifest2"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(2),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app2",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			Expected: AppVersionEnvironments{
				"app1": {
					"1.0": {"dev", "staging"},
					"2.0": {"dev"},
				},
				"app2": {
					"1.0": {"dev"},
				},
			},
		},
		{
			Name: "Retrieve only latest manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manifest2"}},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "app1",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			Expected: AppVersionEnvironments{
				"app1": {
					"1.0": {"dev"},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				manifests, err := dbHandler.DBSelectAllEnvironmentsForAllReleases(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, manifests); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestDBWriteReadUnsynced(t *testing.T) {
	tcs := []struct {
		Name               string
		SyncDataToWrite    []GitSyncData
		ExpectedSyncEvents map[int][]EnvApp
	}{
		{
			Name: "Read Unsynced apps for transformer",
			SyncDataToWrite: []GitSyncData{
				{
					AppName:       "app-1",
					EnvName:       "env-1",
					TransformerID: EslVersion(1),
					SyncStatus:    UNSYNCED,
				},
				{
					AppName:       "app-2",
					EnvName:       "env-1",
					TransformerID: EslVersion(2),
					SyncStatus:    UNSYNCED,
				},
				{
					AppName:       "app-3",
					EnvName:       "env-3",
					TransformerID: EslVersion(3),
					SyncStatus:    UNSYNCED,
				},
				{
					AppName:       "app-4",
					EnvName:       "env-3",
					TransformerID: EslVersion(3),
					SyncStatus:    UNSYNCED,
				},
			},
			ExpectedSyncEvents: map[int][]EnvApp{
				1: {
					{
						AppName: "app-1",
						EnvName: "env-1",
					},
				},
				2: {
					{
						AppName: "app-2",
						EnvName: "env-1",
					},
				},
				3: {
					{
						AppName: "app-3",
						EnvName: "env-3",
					},
					{
						AppName: "app-4",
						EnvName: "env-3",
					},
				},
			},
		},
		{
			Name: "Read Unsynced apps for transformer, some synced",
			SyncDataToWrite: []GitSyncData{
				{
					AppName:       "app-1",
					EnvName:       "env-1",
					TransformerID: EslVersion(1),
					SyncStatus:    SYNCED,
				},
				{
					AppName:       "app-2",
					EnvName:       "env-1",
					TransformerID: EslVersion(2),
					SyncStatus:    SYNCED,
				},
				{
					AppName:       "app-3",
					EnvName:       "env-3",
					TransformerID: EslVersion(3),
					SyncStatus:    UNSYNCED,
				},
				{
					AppName:       "app-4",
					EnvName:       "env-3",
					TransformerID: EslVersion(3),
					SyncStatus:    UNSYNCED,
				},
			},
			ExpectedSyncEvents: map[int][]EnvApp{
				3: {
					{
						AppName: "app-3",
						EnvName: "env-3",
					},
					{
						AppName: "app-4",
						EnvName: "env-3",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, currentSyncData := range tc.SyncDataToWrite {
					err := dbHandler.DBWriteNewSyncEvent(ctx, transaction, &currentSyncData)
					if err != nil {
						return fmt.Errorf("error while writing currentSyncData, error: %w", err)
					}
				}
				for tId, currentExpecteSyncStatus := range tc.ExpectedSyncEvents {
					changes, err := dbHandler.DBReadUnsyncedAppsForTransfomerID(ctx, transaction, TransformerID(tId))
					if err != nil {
						return fmt.Errorf("error while writing currentSyncData, error: %w", err)
					}
					if diff := cmp.Diff(currentExpecteSyncStatus, changes); diff != "" {
						t.Fatalf("error mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}
		})
	}
}

func TestBulkUpdateUnsynced(t *testing.T) {
	tcs := []struct {
		Name                string
		UnsyncedData        []EnvApp
		TargetTransformerID int
	}{
		{
			Name:                "Update all from SYNCED to Unsynced",
			TargetTransformerID: 0,
			UnsyncedData: []EnvApp{
				{
					AppName: "app-1",
					EnvName: "env-1",
				},
				{
					AppName: "app-2",
					EnvName: "env-1",
				},
				{
					AppName: "app-3",
					EnvName: "env-3",
				},
				{
					AppName: "app-4",
					EnvName: "env-3",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				err := dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, TransformerID(0), tc.UnsyncedData, UNSYNCED)
				if err != nil {
					return err
				}
				err = dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, TransformerID(0), SYNCED)
				if err != nil {
					return err
				}
				for _, curr := range tc.UnsyncedData {
					status, err := dbHandler.DBRetrieveSyncStatus(ctx, transaction, curr.AppName, curr.EnvName)
					if err != nil {
						return err
					}
					if status == nil || status.SyncStatus != SYNCED {
						t.Fatalf("UNSYNCED app should be SYNCED: Appname: %q, Envname: %q", curr.AppName, curr.EnvName)
					}
				}

				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction error: %v", err)
			}
		})
	}
}

func TestBulkInsertFunction(t *testing.T) {
	tcs := []struct {
		Name                 string
		ExpectedNumberOfApps int
		BatchSize            int
		expectedError        error
	}{
		{
			Name:                 "Insert No apps",
			ExpectedNumberOfApps: 0,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "one app",
			ExpectedNumberOfApps: 1,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "One batch, just shy",
			ExpectedNumberOfApps: BULK_INSERT_BATCH_SIZE - 1,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "Just enough",
			ExpectedNumberOfApps: BULK_INSERT_BATCH_SIZE,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "Two batches, one too many",
			ExpectedNumberOfApps: BULK_INSERT_BATCH_SIZE + 1,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "Many apps batches",
			ExpectedNumberOfApps: 15*BULK_INSERT_BATCH_SIZE + 1,
			BatchSize:            BULK_INSERT_BATCH_SIZE,
		},
		{
			Name:                 "Batch size 0",
			ExpectedNumberOfApps: BULK_INSERT_BATCH_SIZE + 1,
			BatchSize:            0,
			expectedError:        errMatcher{msg: "batch size needs to be a positive number"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				n := 0
				envApps := make([]EnvApp, 0)
				for n < tc.ExpectedNumberOfApps {
					appName := "app-" + strconv.Itoa(n)
					envName := types.EnvName("env-" + strconv.Itoa(n))
					envApps = append(envApps, EnvApp{AppName: appName, EnvName: envName})
					n += 1
				}
				err := dbHandler.executeBulkInsert(ctx, transaction, envApps, time.Now(), TransformerID(0), UNSYNCED, tc.BatchSize)
				if err != nil {
					if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
						t.Fatalf("error mismatch (-want, +got):\n%s", diff)
					}
					return nil
				} else {
					if tc.expectedError != nil {
						t.Fatalf("expected error but got none\n")
					}
				}

				apps, err := dbHandler.DBReadUnsyncedAppsForTransfomerID(ctx, transaction, TransformerID(0))
				if err != nil {
					t.Fatalf("did not expect error here but got\n %s", err)
				}

				if diff := cmp.Diff(tc.ExpectedNumberOfApps, len(apps)); diff != "" {
					t.Fatalf("mismatch number of apps (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction error: %v", err)
			}
		})
	}
}

func TestBulkReadUnsynced(t *testing.T) {
	tcs := []struct {
		Name                string
		UnsyncedData        map[TransformerID][]EnvApp
		TargetTransformerID int
	}{
		{
			Name:                "All for one transformer",
			TargetTransformerID: 0,
			UnsyncedData: map[TransformerID][]EnvApp{
				0: {
					{
						AppName: "app-1",
						EnvName: "env-1",
					},
					{
						AppName: "app-2",
						EnvName: "env-1",
					},
					{
						AppName: "app-3",
						EnvName: "env-3",
					},
					{
						AppName: "app-4",
						EnvName: "env-3",
					},
				},
			},
		},
		{
			Name:                "Split",
			TargetTransformerID: 0,
			UnsyncedData: map[TransformerID][]EnvApp{
				0: {
					{
						AppName: "app-1",
						EnvName: "env-1",
					},
					{
						AppName: "app-2",
						EnvName: "env-1",
					},
				},
				1: {
					{
						AppName: "app-3",
						EnvName: "env-3",
					},
					{
						AppName: "app-4",
						EnvName: "env-3",
					},
				},
			},
		},
		{
			Name:                "Maps to no transformer",
			TargetTransformerID: 3,
			UnsyncedData: map[TransformerID][]EnvApp{ //transformer ID -> EnvApp
				0: {
					{
						AppName: "app-1",
						EnvName: "env-1",
					},
					{
						AppName: "app-2",
						EnvName: "env-1",
					},
				},
				1: {
					{
						AppName: "app-3",
						EnvName: "env-3",
					},
					{
						AppName: "app-4",
						EnvName: "env-3",
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for tId, currEnvApps := range tc.UnsyncedData {
					err := dbHandler.DBWriteNewSyncEventBulk(ctx, transaction, TransformerID(tId), currEnvApps, UNSYNCED)
					if err != nil {
						return err
					}
					apps, err := dbHandler.DBReadUnsyncedAppsForTransfomerID(ctx, transaction, TransformerID(tId))
					if err != nil {
						return err
					}
					if diff := cmp.Diff(currEnvApps, apps); diff != "" {
						return fmt.Errorf("unsynced apps mismatch (-want +got):\n%s", diff)
					}
				}

				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction error: %v", err)
			}
		})
	}
}

func TestFindEnvAppsFromReleases(t *testing.T) {
	type TestCase struct {
		Name             string
		Releases         []DBReleaseWithMetaData
		ExpectedEnvsApps map[types.EnvName][]string
	}
	tcs := []TestCase{
		{
			Name: "Simple test: several releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
							"env2": "another test manifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app2",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app3",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[types.EnvName][]string{
				"env1": {"app1", "app2", "app3"},
				"env2": {"app1"},
				"env3": {"app1", "app2"},
			},
		},
		{
			Name: "Several Releases for one app",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
							"env2": "another test manifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(11),
						Revision: 0,
					},
					Created: time.Now(),
					App:     "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(12),
						Revision: 0,
					},
					Created: time.Now(),
					App:     "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env4": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[types.EnvName][]string{
				"env1": {"app1"},
				"env2": {"app1"},
				"env3": {"app1"},
				"env4": {"app1"},
			},
		},
		{
			Name: "Releases with different esl versions",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env1": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env2": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					Created:        time.Now(),
					App:            "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[types.EnvName]string{
							"env3": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[types.EnvName][]string{
				"env3": {"app1"},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return err
					}
				}

				envsAppsFromReleases, err := dbHandler.FindEnvsAppsFromReleases(ctx, transaction)
				if err != nil {
					return err
				}
				for env := range envsAppsFromReleases {
					sort.Strings(envsAppsFromReleases[env])
				}
				if diff := cmp.Diff(tc.ExpectedEnvsApps, envsAppsFromReleases); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction, error: %v", err)
			}
		})
	}
}

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *DBHandler {
	ctx := context.Background()
	dir, err := CreateMigrationsPath(2)
	if err != nil {
		t.Fatalf("CreateMigrationsPath: %v", err)
	}
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)

	dbConfig, err := ConnectToPostgresContainer(ctx, t, dir, false, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	migErr := RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}

func SetupRepositoryTestWithDB(t *testing.T, runMigrations bool) (*DBHandler, *DBConfig) {
	migrationsPath, err := CreateMigrationsPath(2)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	return SetupRepositoryTestWithDBMigrationPath(t, migrationsPath, runMigrations)
}

func SetupRepositoryTestWithDBMigrationPath(t *testing.T, migrationsPath string, runMigrations bool) (*DBHandler, *DBConfig) {
	ctx := context.Background()
	dbConfig, err := ConnectToPostgresContainer(ctx, t, migrationsPath, false, t.Name())
	if err != nil {
		t.Fatalf("error connceting %v", err)
		return nil, nil
	}
	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Fatalf("error starting %v", err)
		return nil, nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil, nil
	}
	t.Logf("test created dir: %s", localDir)

	if runMigrations {
		migErr := RunDBMigrations(ctx, *dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}
	}

	dbHandler, err := Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	return dbHandler, dbConfig
}

func TestReadReleasesWithoutEnvironments(t *testing.T) {

	tcs := []struct {
		Name        string
		Releases    []DBReleaseWithMetaData
		AppName     string
		Expected    []*DBReleaseWithMetaData
		ExpectedErr error
	}{
		{
			Name: "Retrieve release without environment",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:       "appNoEnv",
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			AppName: "appNoEnv",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  uversion(1),
						Revision: 0,
					},
					App:          "appNoEnv",
					Manifests:    DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
					Environments: []types.EnvName{},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.Releases {
					err := dbHandler.DBInsertReleaseWithoutEnvironment(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				releases, err := dbHandler.DBSelectReleasesWithoutEnvironments(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, releases, cmpopts.IgnoreFields(DBReleaseWithMetaData{}, "Created")); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestDBSelectAllEnvLocksOfAllApps(t *testing.T) {

	tcs := []struct {
		Name             string
		EnvironmentLocks []EnvironmentLock
		Expected         map[types.EnvName][]EnvironmentLock
	}{
		{
			Name: "Retrieve All Environment locks",
			EnvironmentLocks: []EnvironmentLock{
				{
					EslVersion: 1,
					Created:    time.Now(),
					LockID:     "lockId1",
					Env:        "development",
					Deleted:    false,
					Metadata: LockMetadata{
						CreatedByName:  "author1",
						CreatedByEmail: "email1",
						Message:        "message1",
						CiLink:         "cilink1",
						CreatedAt:      time.Now(),
					},
				},
				{
					EslVersion: 1,
					Created:    time.Now(),
					LockID:     "lockId2",
					Env:        "staging",
					Deleted:    false,
					Metadata: LockMetadata{
						CreatedByName:  "author2",
						CreatedByEmail: "email2",
						Message:        "message2",
						CiLink:         "cilink2",
						CreatedAt:      time.Now(),
					},
				},
			},
			Expected: map[types.EnvName][]EnvironmentLock{
				"development": {
					{
						EslVersion: 1,
						Created:    time.Now(),
						LockID:     "lockId1",
						Env:        "development",
						Deleted:    false,
						Metadata: LockMetadata{
							CreatedByName:  "author1",
							CreatedByEmail: "email1",
							Message:        "message1",
							CiLink:         "cilink1",
							CreatedAt:      time.Now(),
						},
					},
				},
				"staging": {
					{
						EslVersion: 1,
						Created:    time.Now(),
						LockID:     "lockId2",
						Env:        "staging",
						Deleted:    false,
						Metadata: LockMetadata{
							CreatedByName:  "author2",
							CreatedByEmail: "email2",
							Message:        "message2",
							CiLink:         "cilink2",
							CreatedAt:      time.Now(),
						},
					},
				},
			},
		},
		{
			Name: "Different esl versions and deleted",
			EnvironmentLocks: []EnvironmentLock{
				{
					EslVersion: 1,
					Created:    time.Now(),
					LockID:     "lockId1",
					Env:        "development",
					Deleted:    false,
					Metadata: LockMetadata{
						CreatedByName:  "author1",
						CreatedByEmail: "email1",
						Message:        "message1",
						CiLink:         "cilink1",
						CreatedAt:      time.Now(),
					},
				},
				{
					EslVersion: 1,
					Created:    time.Now(),
					LockID:     "lockId2",
					Env:        "staging",
					Deleted:    false,
					Metadata: LockMetadata{
						CreatedByName:  "author2",
						CreatedByEmail: "email2",
						Message:        "message2",
						CiLink:         "cilink2",
						CreatedAt:      time.Now(),
					},
				},
				{
					EslVersion: 2,
					Created:    time.Now(),
					LockID:     "lockId1",
					Env:        "development",
					Deleted:    false,
					Metadata: LockMetadata{
						CreatedByName:  "author3",
						CreatedByEmail: "email3",
						Message:        "message3",
						CiLink:         "cilink3",
						CreatedAt:      time.Now(),
					},
				},
				{
					EslVersion: 1,
					Created:    time.Now(),
					LockID:     "lockId4",
					Env:        "development",
					Deleted:    true,
					Metadata: LockMetadata{
						CreatedByName:  "author4",
						CreatedByEmail: "email4",
						Message:        "message4",
						CiLink:         "cilink4",
						CreatedAt:      time.Now(),
					},
				},
			},
			Expected: map[types.EnvName][]EnvironmentLock{
				"development": {
					{
						EslVersion: 2,
						Created:    time.Now(),
						LockID:     "lockId1",
						Env:        "development",
						Deleted:    false,
						Metadata: LockMetadata{
							CreatedByName:  "author3",
							CreatedByEmail: "email3",
							Message:        "message3",
							CiLink:         "cilink3",
							CreatedAt:      time.Now(),
						},
					},
				},
				"staging": {
					{
						EslVersion: 1,
						Created:    time.Now(),
						LockID:     "lockId2",
						Env:        "staging",
						Deleted:    false,
						Metadata: LockMetadata{
							CreatedByName:  "author2",
							CreatedByEmail: "email2",
							Message:        "message2",
							CiLink:         "cilink2",
							CreatedAt:      time.Now(),
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, envLock := range tc.EnvironmentLocks {
					err := dbHandler.DBWriteEnvironmentLockInternal(ctx, transaction, envLock, envLock.EslVersion-1)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				envLocks, err := dbHandler.DBSelectAllEnvLocksOfAllEnvs(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, envLocks, cmpopts.IgnoreFields(EnvironmentLock{}, "Created"), cmpopts.IgnoreFields(LockMetadata{}, "CreatedAt")); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestDBSelectAllTeamLocksOfAllEnvs(t *testing.T) {

	tcs := []struct {
		Name      string
		TeamLocks []TeamLock
		Expected  map[types.EnvName]map[string][]TeamLock
	}{
		{
			Name: "Retrieve All Environment locks",
			TeamLocks: []TeamLock{
				{
					Created: time.Now(),
					LockID:  "lockId1",
					Env:     "development",
					Team:    "team1",
					Metadata: LockMetadata{
						CreatedByName:  "author1",
						CreatedByEmail: "email1",
						Message:        "message1",
						CiLink:         "cilink1",
						CreatedAt:      time.Now(),
					},
				},
				{
					Created: time.Now(),
					LockID:  "lockId3",
					Env:     "development",
					Team:    "team2",
					Metadata: LockMetadata{
						CreatedByName:  "author3",
						CreatedByEmail: "email3",
						Message:        "message3",
						CiLink:         "cilink3",
						CreatedAt:      time.Now(),
					},
				},
				{
					Created: time.Now(),
					LockID:  "lockId2",
					Env:     "staging",
					Team:    "team2",
					Metadata: LockMetadata{
						CreatedByName:  "author2",
						CreatedByEmail: "email2",
						Message:        "message2",
						CiLink:         "cilink2",
						CreatedAt:      time.Now(),
					},
				},
			},
			Expected: map[types.EnvName]map[string][]TeamLock{
				"development": {
					"team1": {
						{
							Created: time.Now(),
							LockID:  "lockId1",
							Env:     "development",
							Team:    "team1",
							Metadata: LockMetadata{
								CreatedByName:  "author1",
								CreatedByEmail: "email1",
								Message:        "message1",
								CiLink:         "cilink1",
								CreatedAt:      time.Now(),
							},
						},
					},
					"team2": {
						{
							Created: time.Now(),
							LockID:  "lockId3",
							Env:     "development",
							Team:    "team2",
							Metadata: LockMetadata{
								CreatedByName:  "author3",
								CreatedByEmail: "email3",
								Message:        "message3",
								CiLink:         "cilink3",
								CreatedAt:      time.Now(),
							},
						},
					},
				},
				"staging": {
					"team2": {
						{
							Created: time.Now(),
							LockID:  "lockId2",
							Env:     "staging",
							Team:    "team2",
							Metadata: LockMetadata{
								CreatedByName:  "author2",
								CreatedByEmail: "email2",
								Message:        "message2",
								CiLink:         "cilink2",
								CreatedAt:      time.Now(),
							},
						},
					},
				},
			},
		},
		{
			Name: "Different esl versions and deleted",
			TeamLocks: []TeamLock{
				{
					Created: time.Now(),
					LockID:  "lockId1",
					Env:     "development",
					Team:    "team1",
					Metadata: LockMetadata{
						CreatedByName:  "author1",
						CreatedByEmail: "email1",
						Message:        "message1",
						CiLink:         "cilink1",
						CreatedAt:      time.Now(),
					},
				},
				{
					Created: time.Now(),
					LockID:  "lockId2",
					Env:     "staging",
					Team:    "team2",
					Metadata: LockMetadata{
						CreatedByName:  "author2",
						CreatedByEmail: "email2",
						Message:        "message2",
						CiLink:         "cilink2",
						CreatedAt:      time.Now(),
					},
				},
				{
					Created: time.Now(),
					LockID:  "lockId1",
					Env:     "development",
					Team:    "team1",
					Metadata: LockMetadata{
						CreatedByName:  "author3",
						CreatedByEmail: "email3",
						Message:        "message3",
						CiLink:         "cilink3",
						CreatedAt:      time.Now(),
					},
				},
			},
			Expected: map[types.EnvName]map[string][]TeamLock{
				"development": {
					"team1": {
						{
							Created: time.Now(),
							LockID:  "lockId1",
							Env:     "development",
							Team:    "team1",
							Metadata: LockMetadata{
								CreatedByName:  "author3",
								CreatedByEmail: "email3",
								Message:        "message3",
								CiLink:         "cilink3",
								CreatedAt:      time.Now(),
							},
						},
					},
				},
				"staging": {
					"team2": {
						{
							Created: time.Now(),
							LockID:  "lockId2",
							Env:     "staging",
							Team:    "team2",
							Metadata: LockMetadata{
								CreatedByName:  "author2",
								CreatedByEmail: "email2",
								Message:        "message2",
								CiLink:         "cilink2",
								CreatedAt:      time.Now(),
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, teamLock := range tc.TeamLocks {
					err := dbHandler.DBWriteTeamLock(ctx, transaction, teamLock.LockID, teamLock.Env, teamLock.Team, teamLock.Metadata)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				teamLocks, err := dbHandler.DBSelectAllTeamLocksOfAllEnvs(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}
				if diff := cmp.Diff(tc.Expected, teamLocks, cmpopts.IgnoreFields(TeamLock{}, "Created"), cmpopts.IgnoreFields(LockMetadata{}, "CreatedAt")); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestDbUpdateAllDeployments(t *testing.T) {
	const oldId = 1
	const newId = 2
	const app = "my-app"

	tcs := []struct {
		Name                     string
		InitialDeployments       []Deployment
		ExpectedDeploymentsNewID []Deployment
		ExpectedDeploymentsOldId []Deployment
	}{
		{
			Name:                     "no deployments",
			InitialDeployments:       []Deployment{},
			ExpectedDeploymentsNewID: []Deployment{},
			ExpectedDeploymentsOldId: []Deployment{},
		},
		{
			Name: "change single deployment",
			InitialDeployments: []Deployment{
				{
					Created: time.Now(),
					Env:     "development",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: oldId,
				},
			},
			ExpectedDeploymentsNewID: []Deployment{
				{
					Created: time.Now(),
					Env:     "development",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: newId,
				},
			},
		},
		{
			Name: "change multiple deployments",
			InitialDeployments: []Deployment{
				{
					Created: time.Now(),
					App:     app,
					Env:     "development-2",
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: oldId,
				},
				{
					Created: time.Now(),
					Env:     "development",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: oldId,
				},
			},
			ExpectedDeploymentsOldId: []Deployment{},
			ExpectedDeploymentsNewID: []Deployment{
				{
					Created: time.Now(),
					Env:     "development-2",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: newId,
				},
				{
					Created: time.Now(),
					Env:     "development",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: newId,
				},
			},
		},
		{
			Name: "change multiple deployments, but with other that should not change",
			InitialDeployments: []Deployment{
				{
					Created: time.Now(),
					App:     app,
					Env:     "development",
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: oldId,
				},
				{
					Created: time.Now(),
					Env:     "development-2",
					App:     "my-app",
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: oldId,
				},
				{
					Created: time.Now(),
					App:     "my-app",
					Env:     "development-3",
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: 3,
				},
			},
			ExpectedDeploymentsNewID: []Deployment{
				{
					Created: time.Now(),
					Env:     "development",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: newId,
				},
				{
					Created: time.Now(),
					Env:     "development-2",
					App:     app,
					Metadata: DeploymentMetadata{
						DeployedByName:  "author1",
						DeployedByEmail: "email1",
						CiLink:          "cilink1",
					},
					TransformerID: newId,
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteEslEventWithJson(ctx, transaction, EvtMigrationTransformer, "{}")
				if err != nil {
					return fmt.Errorf("error while writing EvtMigrationTransformer, error: %w", err)
				}
				err = dbHandler.DBWriteEslEventWithJson(ctx, transaction, EvtMigrationTransformer, "{}")
				if err != nil {
					return fmt.Errorf("error while writing EvtMigrationTransformer, error: %w", err)
				}
				err = dbHandler.DBWriteEslEventWithJson(ctx, transaction, EvtMigrationTransformer, "{}")
				if err != nil {
					return fmt.Errorf("error while writing EvtMigrationTransformer, error: %w", err)
				}
				for _, deployment := range tc.InitialDeployments {
					err := dbHandler.upsertDeploymentRow(ctx, transaction, deployment)
					if err != nil {
						return fmt.Errorf("error while writing deployment, error: %w", err)
					}
				}

				err = dbHandler.DBBulkUpdateAllDeployments(ctx, transaction, newId, oldId)

				if err != nil {
					return fmt.Errorf("error while writing deployments, error: %w", err)
				}

				actualDeploymentsNewId, err := dbHandler.DBSelectDeploymentsByTransformerID(ctx, transaction, newId)
				if err != nil {
					return fmt.Errorf("error while reading deployments, error: %w", err)
				}

				if diff := cmp.Diff(tc.ExpectedDeploymentsNewID, actualDeploymentsNewId, cmpopts.IgnoreFields(Deployment{}, "Created")); diff != "" {
					return fmt.Errorf("deployments with new ID mismatch (-want +got):\n%s", diff)
				}

				actualDeploymentsOldId, err := dbHandler.DBSelectDeploymentsByTransformerID(ctx, transaction, oldId)
				if err != nil {
					return fmt.Errorf("error while reading deployments, error: %w", err)
				}

				if len(actualDeploymentsOldId) != 0 {
					return fmt.Errorf("no deployments should have old transformer ID, got \n%v", actualDeploymentsOldId)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

		})
	}
}

func TestDBSelectEnvironmentApplications(t *testing.T) {
	tcs := []struct {
		Name                            string
		Releases                        []DBReleaseWithMetaData
		Environments                    []DBEnvironment
		ExpectedEnvironmentApplications map[types.EnvName][]string
	}{
		{
			Name: "one Release",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manifest2"}},
				},
			},
			Environments: []DBEnvironment{
				{
					Name:   "dev",
					Config: config.EnvironmentConfig{},
				},
				{
					Name:   "staging",
					Config: config.EnvironmentConfig{},
				},
			},
			ExpectedEnvironmentApplications: map[types.EnvName][]string{
				"dev":     {"app1"},
				"staging": {"app1"},
			},
		},
		{
			Name: "One environment without apps",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			Environments: []DBEnvironment{
				{
					Name:   "dev",
					Config: config.EnvironmentConfig{},
				},
			},
			ExpectedEnvironmentApplications: map[types.EnvName][]string{
				"dev":     {"app1"},
				"staging": {},
			},
		},
		{
			Name: "Multiple releases and environments",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "production": "manifest3"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"staging": "manifest2"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app3",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app3",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"production": "manifest3"}},
				},
			},
			Environments: []DBEnvironment{
				{
					Name:   "dev",
					Config: config.EnvironmentConfig{},
				},
				{
					Name:   "staging",
					Config: config.EnvironmentConfig{},
				},
				{
					Name:   "production",
					Config: config.EnvironmentConfig{},
				},
			},
			ExpectedEnvironmentApplications: map[types.EnvName][]string{
				"dev":        {"app1", "app2", "app3"},
				"staging":    {"app1"},
				"production": {"app1", "app3"},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, environment := range tc.Environments {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, environment.Name, environment.Config, make([]string, 0))
					if err != nil {
						return fmt.Errorf("error while writing environment, error: %w", err)
					}
				}
				for _, release := range tc.Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				for envName, expectedApps := range tc.ExpectedEnvironmentApplications {
					apps, err := dbHandler.DBSelectEnvironmentApplications(ctx, transaction, envName)
					if err != nil {
						return fmt.Errorf("Couldn't retrieve environment %s applications, error: %w", envName, err)
					}
					if diff := cmp.Diff(expectedApps, apps); diff != "" {
						return fmt.Errorf("environment applications mismatch (-want, +got):\n%s", diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestDBSelectEnvironmentApplicationsAtTimestamp(t *testing.T) {
	tcs := []struct {
		Name                            string
		FirstReleases                   []DBReleaseWithMetaData
		SecondReleases                  []DBReleaseWithMetaData
		Environments                    []DBEnvironment
		ExpectedEnvironmentApplications map[types.EnvName][]string // this related to the apps BEFORE the 2nd releases
	}{
		{
			Name: "Environment added afterwards",
			FirstReleases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
			},
			SecondReleases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"staging": "manifest2"}},
				},
			},
			Environments: []DBEnvironment{
				{
					Name:   "dev",
					Config: config.EnvironmentConfig{},
				},
				{
					Name:   "staging",
					Config: config.EnvironmentConfig{},
				},
			},
			ExpectedEnvironmentApplications: map[types.EnvName][]string{
				"dev":     {"app1"},
				"staging": {},
			},
		},
		{
			Name: "New app added afterwards",
			FirstReleases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(20),
					App:            "app1",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"staging": "manifest2"}},
				},
			},
			SecondReleases: []DBReleaseWithMetaData{
				{
					ReleaseNumbers: types.MakeReleaseNumberVersion(10),
					App:            "app2",
					Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{"dev": "manifest1", "staging": "manifest2"}},
				},
			},
			Environments: []DBEnvironment{
				{
					Name:   "dev",
					Config: config.EnvironmentConfig{},
				},
				{
					Name:   "staging",
					Config: config.EnvironmentConfig{},
				},
			},
			ExpectedEnvironmentApplications: map[types.EnvName][]string{
				"dev":     {"app1"},
				"staging": {"app1"},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			firstReleaseTime, err := WithTransactionT(dbHandler, ctx, 1, false, func(ctx context.Context, transaction *sql.Tx) (*time.Time, error) {
				for _, environment := range tc.Environments {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, environment.Name, environment.Config, make([]string, 0))
					if err != nil {
						return nil, fmt.Errorf("error while writing environment, error: %w", err)
					}
				}
				for _, release := range tc.FirstReleases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return nil, fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				firstReleaseTime, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return nil, err
				}
				return firstReleaseTime, nil
			})
			if err != nil {
				t.Fatalf("error with firstrelease: %v", err)
			}
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, release := range tc.SecondReleases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				secondReleaseTime, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return err
				}
				if secondReleaseTime == nil || firstReleaseTime.Compare(*secondReleaseTime) != -1 {
					return fmt.Errorf("the second timestamp must be later than the first: t1=%v; t2=%v", firstReleaseTime, secondReleaseTime)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error with second release: %v", err)
			}

			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for envName, expectedApps := range tc.ExpectedEnvironmentApplications {
					apps, err := dbHandler.DBSelectEnvironmentApplicationsAtTimestamp(ctx, transaction, envName, *firstReleaseTime)
					if err != nil {
						return fmt.Errorf("Couldn't retrieve environment %s applications, error: %w", envName, err)
					}
					if diff := cmp.Diff(expectedApps, apps); diff != "" {
						return fmt.Errorf("environment applications mismatch for env '%s' (-want, +got):\n%s", envName, diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error with check: %v", err)
			}
		})
	}
}
func TestDBSelectCommitIdAppReleaseVersions(t *testing.T) {
	tcs := []struct {
		Name     string
		App      string
		Version  types.ReleaseNumbers
		CommitId string
		Repeats  uint
	}{
		{
			Name:     "One app",
			App:      "foo",
			Version:  types.MakeReleaseNumberVersion(10),
			CommitId: "deadbeefdeadbeefdeaddeadbeefdeadbeefdead",
			Repeats:  3,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envName := types.EnvName("env")
				err := dbHandler.DBWriteEnvironment(
					ctx,
					transaction,
					envName,
					config.EnvironmentConfig{},
					make([]string, 0),
				)
				if err != nil {
					return fmt.Errorf("error while writing environment, error: %w", err)
				}
				metadata := DBReleaseMetaData{
					SourceCommitId: tc.CommitId,
				}
				release := DBReleaseWithMetaData{
					ReleaseNumbers: types.ReleaseNumbers{
						Version:  tc.Version.Version,
						Revision: tc.Version.Revision,
					},
					App:       tc.App,
					Manifests: DBReleaseManifests{Manifests: map[types.EnvName]string{envName: "manifest1"}},
					Metadata:  metadata,
				}
				err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
				if err != nil {
					return fmt.Errorf("error while writing release, error: %w", err)
				}
				for i := tc.Repeats; i > 0; i-- {
					versionByApp := make(map[string]types.ReleaseNumbers)
					versionByApp[tc.App] = tc.Version
					commitIdByApp, err := dbHandler.DBSelectCommitIdAppReleaseVersions(ctx, transaction, versionByApp)
					if err != nil {
						return fmt.Errorf("error while retriving commit id, error: %w", err)
					}
					if len(commitIdByApp) != len(versionByApp) {
						return fmt.Errorf("commit id map len mismatches len of commitByApp. expected: %v, got: %v", len(versionByApp), len(commitIdByApp))
					}
					if commitIdByApp[tc.App] != tc.CommitId {
						return fmt.Errorf("Did not find expected commit id for %v. expected: %v, got: %v", tc.App, tc.CommitId, commitIdByApp[tc.App])
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error with check: %v", err)
			}
		})
	}
}

func TestDBSelectCommitIdAppReleaseVersionsMany(t *testing.T) {
	tcs := []struct {
		Name      string
		AppPrefix string
		Apps      uint
	}{
		{
			Name:      "Big Query app",
			AppPrefix: "foo",
			Apps:      10000,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				envName := types.EnvName("env")
				err := dbHandler.DBWriteEnvironment(
					ctx,
					transaction,
					envName,
					config.EnvironmentConfig{},
					make([]string, 0),
				)
				if err != nil {
					return fmt.Errorf("error while writing environment, error: %w", err)
				}
				versionByApp := make(map[string]types.ReleaseNumbers)
				for i := tc.Apps; i > 0; i-- {
					appName := fmt.Sprintf("%s%d", tc.AppPrefix, i)
					versionByApp[appName] = types.MakeReleaseNumberVersion(uint64(i + 20000))
					metadata := DBReleaseMetaData{
						SourceCommitId: fmt.Sprintf("%d", i),
					}
					release := DBReleaseWithMetaData{
						ReleaseNumbers: versionByApp[appName],
						App:            fmt.Sprintf("%s%d", tc.AppPrefix, i),
						Manifests:      DBReleaseManifests{Manifests: map[types.EnvName]string{envName: "manifest1"}},
						Metadata:       metadata,
					}
					err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}

				commitIdByApp, err := dbHandler.DBSelectCommitIdAppReleaseVersions(ctx, transaction, versionByApp)
				if err != nil {
					return fmt.Errorf("error while retriving commit id, error: %w", err)
				}
				if len(commitIdByApp) != len(versionByApp) {
					return fmt.Errorf("commit id map len mismatches len of commitByApp. expected: %v, got: %v", len(versionByApp), len(commitIdByApp))
				}
				for i := tc.Apps; i > 0; i-- {
					appName := fmt.Sprintf("%s%d", tc.AppPrefix, i)
					if commitIdByApp[appName] != fmt.Sprintf("%d", i) {
						return fmt.Errorf("Did not find expected commit id. expected: %v, got: %v", fmt.Sprintf("%d", i), commitIdByApp[appName])
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error with check: %v", err)
			}
		})
	}
}

// DBInsertReleaseWithoutEnvironment inserts a release with mismatching manifest and environments into the database.
// This behaviour is intended to test the `DBSelectReleasesWithoutEnvironments` method which exists only for migration purposes.
func (h *DBHandler) DBInsertReleaseWithoutEnvironment(ctx context.Context, transaction *sql.Tx, release DBReleaseWithMetaData) error {
	metadataJson, err := json.Marshal(release.Metadata)
	if err != nil {
		return fmt.Errorf("insert release: could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery(
		"INSERT INTO releases (created, releaseVersion, appName, manifests, metadata, environments)  VALUES (?, ?, ?, ?, ?, ?);",
	)
	manifestJson, err := json.Marshal(release.Manifests)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	environmentStr := ""

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("DBInsertRelease unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		insertQuery,
		*now,
		*release.ReleaseNumbers.Version,
		release.App,
		manifestJson,
		metadataJson,
		environmentStr,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w",
			release.App,
			*release.ReleaseNumbers.Version,
			err)
	}

	return nil
}
