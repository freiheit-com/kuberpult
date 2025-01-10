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
	"errors"
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

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
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

func TestConnection(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "Ping DB",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			t.Parallel()
			dir := t.TempDir()
			cfg := DBConfig{
				DriverName: "sqlite3",
				DbHost:     dir,
			}
			db, err := Connect(ctx, cfg)
			if err != nil {
				t.Fatalf("Error establishing DB connection. Error: %v\n", err)
			}
			defer db.DB.Close()
			pingErr := db.DB.Ping()
			if pingErr != nil {
				t.Fatalf("Error DB. Error: %v\n", err)
			}
		})
	}
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

INSERT INTO apps (created, appname, statechange, metadata)  VALUES ('1713218400', 'my-test-app', 'AppStateChangeMigrate', '{}');`,
			expectedData: []string{"my-test-app"},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbDir := t.TempDir()
			cfg := DBConfig{
				DriverName:     "sqlite3",
				DbHost:         dbDir,
				MigrationsPath: dbDir + "/migrations",
			}
			loc, mkdirErr := createMigrationFolder(dbDir)
			if mkdirErr != nil {
				t.Fatalf("Error creating migrations folder. Error: %v\n", mkdirErr)
			}

			ts := time.Now().Unix()
			migrationFileNameAbsPath := path.Join(loc, strconv.FormatInt(ts, 10)+"_testing.up.sql")
			wErr := os.WriteFile(migrationFileNameAbsPath, []byte(tc.migrationFile), os.ModePerm)
			if wErr != nil {
				t.Fatalf("Error creating migration file. Error: %v\n", mkdirErr)
			}

			migErr := RunDBMigrations(ctx, cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := Connect(ctx, cfg)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}
			tx, err := db.BeginTransaction(ctx, false)
			if err != nil {
				t.Fatalf("Error creating transaction. Error: %v\n", err)
			}
			m, err := db.DBSelectAllApplications(ctx, tx)
			if err != nil {
				t.Fatalf("Error querying dabatabse. Error: %v\n", err)
			}
			err = tx.Commit()
			if err != nil {
				t.Fatalf("Error commiting transaction. Error: %v\n", err)
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
				Manifests: map[string]string{
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
				Manifests: map[string]string{
					"dev": "manifest2",
				},
			},
		}
		for _, r := range releases {
			dbRelease := DBReleaseWithMetaData{
				Created:       time.Now().UTC(),
				ReleaseNumber: r.Version,
				App:           app,
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
					ReleaseNumber: 666,
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"dev": "manifest1",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   "auth1",
						SourceCommitId: "commit1",
						SourceMessage:  "msg1",
						DisplayVersion: "display1",
					},
					Environments: []string{"dev"},
				},
				{
					ReleaseNumber: 777,
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"dev": "manifest2",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   "auth2",
						SourceCommitId: "commit2",
						SourceMessage:  "msg2",
						DisplayVersion: "display2",
					},
					Environments: []string{"dev"},
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dbHandler := SetupRepositoryTestWithDB(t)
			err3 := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.RunCustomMigrationReleases(ctx, getAllApps, writeAllReleases)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				for i := range tc.expectedReleases {
					expectedRelease := tc.expectedReleases[i]

					release, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, expectedRelease.App, expectedRelease.ReleaseNumber, true)
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

			dbHandler := SetupRepositoryTestWithDB(t)
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

			dbHandler := SetupRepositoryTestWithDB(t)
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
					t.Errorf("could not get migration event: %v\n", err)

				}
				if !contains {
					t.Errorf("migration event was not created: %v\n", err)
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

			dir, err := testutil.CreateMigrationsPath(2)
			if err != nil {
				t.Fatalf("setup error could not detect dir \n%v", err)
				return
			}

			cfg := DBConfig{
				DriverName:     "sqlite3",
				DbHost:         dbDir,
				MigrationsPath: dir,
			}
			migErr := RunDBMigrations(ctx, cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := Connect(ctx, cfg)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}

			err = db.RunCustomMigrationsEventSourcingLight(ctx)
			if err != nil {
				t.Fatalf("Error running custom migrations for esl table. Error: %v\n", err)

			}
			err = db.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				if err != nil {
					t.Fatalf("Error creating transaction. Error: %v\n", err)
				}
				err := writeEventAux(ctx, db, tx, tc.commitHash, tc.event)
				if err != nil {
					t.Fatalf("Error writing event to DB. Error: %v\n", err)
				}

				m, err := db.DBSelectAllEventsForCommit(ctx, tx, tc.commitHash, 0, 100)
				if err != nil {
					t.Fatalf("Error querying dabatabse. Error: %v\n", err)
				}
				for _, currEvent := range m {
					e, err := event.UnMarshallEvent(event.EventType(tc.event.EventMetadata.EventType), currEvent.EventJson)

					if err != nil {
						t.Fatalf("Error obtaining event from DB. Error: %v\n", err)
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

			dir, err := testutil.CreateMigrationsPath(2)
			if err != nil {
				t.Fatalf("setup error could not detect dir \n%v", err)
				return
			}

			cfg := DBConfig{
				DriverName:     "sqlite3",
				DbHost:         dbDir,
				MigrationsPath: dir,
			}
			migErr := RunDBMigrations(ctx, cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := Connect(ctx, cfg)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}

			err = db.RunCustomMigrationsEventSourcingLight(ctx)
			if err != nil {
				t.Fatalf("Error running custom migrations for esl table. Error: %v\n", err)

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

func TestReadWriteDeployment(t *testing.T) {
	tcs := []struct {
		Name               string
		App                string
		Env                string
		VersionToDeploy    *int64
		ExpectedDeployment *Deployment
	}{
		{
			Name:            "with eslVersion != nil",
			App:             "app-a",
			Env:             "dev",
			VersionToDeploy: version(7),
			ExpectedDeployment: &Deployment{
				App:           "app-a",
				Env:           "dev",
				Version:       version(7),
				TransformerID: 0,
			},
		},
		{
			Name:            "with eslVersion == nil",
			App:             "app-b",
			Env:             "prod",
			VersionToDeploy: nil,
			ExpectedDeployment: &Deployment{
				App:           "app-b",
				Env:           "prod",
				Version:       nil,
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *deployment))
				}

				err := dbHandler.DBWriteMigrationsTransformer(ctx, transaction)
				if err != nil {
					return err
				}

				err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, Deployment{
					App:           tc.App,
					Env:           tc.Env,
					Version:       tc.VersionToDeploy,
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
		ExpectedDeployments map[string]Deployment
	}{
		{
			Name:    "Select one deployment",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]Deployment{
				"dev": {
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Select only latest deployment",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(6),
					TransformerID: 0,
				},
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]Deployment{
				"dev": {
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
			},
		},
		{
			Name:    "Select multiple deployments",
			AppName: "app1",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(6),
					TransformerID: 0,
				},
				{
					App:           "app1",
					Env:           "staging",
					Version:       version(5),
					TransformerID: 0,
				},
				{
					App:           "app2",
					Env:           "staging",
					Version:       version(5),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]Deployment{
				"dev": {
					App:           "app1",
					Env:           "dev",
					Version:       version(6),
					TransformerID: 0,
				},
				"staging": {
					App:           "app1",
					Env:           "staging",
					Version:       version(5),
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
		EnvName             string
		SetupDeployments    []*Deployment
		ExpectedDeployments map[string]*int64
	}{
		{
			Name:    "Select one deployment",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]*int64{
				"app1": version(7),
			},
		},
		{
			Name:    "Select latest deployment",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(8),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]*int64{
				"app1": version(8),
			},
		},
		{
			Name:    "Select multiple deployments",
			EnvName: "dev",
			SetupDeployments: []*Deployment{
				{
					App:           "app1",
					Env:           "dev",
					Version:       version(7),
					TransformerID: 0,
				},
				{
					App:           "app2",
					Env:           "dev",
					Version:       version(8),
					TransformerID: 0,
				},
				{
					App:           "app3",
					Env:           "staging",
					Version:       version(8),
					TransformerID: 0,
				},
			},
			ExpectedDeployments: map[string]*int64{
				"app1": version(7),
				"app2": version(8),
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
		Env           string
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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
		EnvName string
		Version uint64
	}
	tcs := []struct {
		Name     string
		AppName  string
		data     []data
		expected map[string]int64
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
			expected: map[string]int64{
				"development": 3,
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
			expected: map[string]int64{
				"development": 3,
				"staging":     2,
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
			expected: map[string]int64{
				"development": 3,
				"staging":     3,
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
			expected: map[string]int64{
				"development": 1,
				"staging":     1,
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
					newVersion := int64(d.Version)
					deployment := Deployment{
						Created:       time.Now(),
						App:           tc.AppName,
						Env:           d.EnvName,
						Version:       &newVersion,
						Metadata:      DeploymentMetadata{},
						TransformerID: 0,
					}
					err := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, deployment)
					if err != nil {
						t.Fatalf("Error updating all deployments: %v\n", err)
					}
				}
				result, err := dbHandler.DBSelectAllDeploymentsForApp(ctx, transaction, tc.AppName)
				if err != nil {
					t.Fatalf("Error reading from all deployments: %v\n", err)
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
		Env          string
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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
		Env          string
		LockID       string
		Message      string
		AppName      string
		AuthorName   string
		AuthorEmail  string
		CiLink       string
		ExpectedLock *ApplicationLock
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
			ExpectedLock: &ApplicationLock{
				Env:        "dev",
				LockID:     "dev-app-lock",
				EslVersion: 1,
				Deleted:    false,
				App:        "my-app",
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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
				if diff := cmp.Diff(tc.ExpectedLock, &target, cmpopts.IgnoreFields(ApplicationLock{}, "Created")); diff != "" {
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
		Env         string
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
					Env:        "dev",
					LockID:     "dev-app-lock",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
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
					Env:        "dev",
					LockID:     "dev-app-lock",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
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
		Env         string
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
					Env:        "dev",
					LockID:     "dev-app-lock",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
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
					Env:        "dev",
					LockID:     "dev-app-lock",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:        "dev",
					LockID:     "dev-app-lock-2",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app-2",
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
					Env:        "dev",
					LockID:     "dev-app-lock",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on dev for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:        "staging",
					LockID:     "dev-app-lock-staging",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app",
					Metadata: LockMetadata{
						Message:        "My application lock on stagibg for my-app",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
						CiLink:         "www.test.com",
					},
				},
				{
					Env:        "staging",
					LockID:     "dev-app-lock-staging-2",
					EslVersion: 1,
					Deleted:    false,
					App:        "my-app-2",
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
		Env         string
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
					Env:        "dev",
					LockID:     "dev-team-lock",
					EslVersion: 1,
					Deleted:    false,
					Team:       "my-team",
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
			ExpectedActiveLocks: nil,
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
					Env:        "dev",
					LockID:     "dev-team-lock",
					EslVersion: 1,
					Deleted:    false,
					Team:       "my-team",
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
		Env           string
		LockID        string
		Message       string
		AppName       string
		AuthorName    string
		AuthorEmail   string
		ExpectedLocks []ApplicationLock
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
			ExpectedLocks: []ApplicationLock{
				{ //Sort DESC
					Env:        "dev",
					App:        "myApp",
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
					App:        "myApp",
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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

				if diff := cmp.Diff(&tc.ExpectedLocks, &actual, cmpopts.IgnoreFields(ApplicationLock{}, "Created")); diff != "" {
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

func TestQueueApplicationVersion(t *testing.T) {
	const envName = "dev"
	const appName = "deployment"

	tcs := []struct {
		Name                string
		Deployments         []QueuedDeployment
		ExpectedDeployments []QueuedDeployment
		Env                 string
		AppName             string
		Version             *int64
	}{
		{
			Name: "Write and read",
			Deployments: []QueuedDeployment{
				{
					Env:     envName,
					App:     appName,
					Version: version(0),
				},
			},
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					Version:    version(0),
				},
			},
		},
		{
			Name: "Write and read multiple",
			Deployments: []QueuedDeployment{
				{
					Env:     envName,
					App:     appName,
					Version: version(0),
				},
				{
					Env:     envName,
					App:     appName,
					Version: version(1),
				},
			},
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 2,
					Env:        envName,
					App:        appName,
					Version:    version(1),
				},
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					Version:    version(0),
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
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.Version, false)
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
		Env                 string
		AppName             string
		Version             *int64
		ExpectedDeployments []QueuedDeployment
	}{
		{
			Name:    "Write and delete",
			Env:     envName,
			AppName: appName,
			Version: version(1),
			ExpectedDeployments: []QueuedDeployment{
				{
					EslVersion: 2,
					Env:        envName,
					App:        appName,
					Version:    nil,
				},
				{
					EslVersion: 1,
					Env:        envName,
					App:        appName,
					Version:    version(1),
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
				err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, tc.Env, tc.AppName, tc.Version, false)
				if err != nil {
					return err
				}

				err = dbHandler.DBDeleteDeploymentAttempt(ctx, transaction, tc.Env, tc.AppName, false)
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
					Env:     "dev",
					App:     appName,
					Version: version(0),
				},
				{
					Env:     "staging",
					App:     appName,
					Version: version(0),
				},
				{
					EslVersion: 2,
					Env:        "staging",
					App:        appName,
					Version:    version(1),
				},
				{
					Env:     "dev",
					App:     "bar",
					Version: version(0),
				},
			},
			ExpectedDeployments: []*QueuedDeployment{
				{
					EslVersion: 1,
					Env:        "dev",
					App:        appName,
					Version:    version(0),
				},
				{
					EslVersion: 2,
					Env:        "staging",
					App:        appName,
					Version:    version(1),
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
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.Version, false)
					if err != nil {
						return err
					}
				}

				queuedDeployments, err := dbHandler.DBSelectLatestDeploymentAttemptOnAllEnvironments(ctx, transaction, tc.App)
				if err != nil {
					return err
				}

				slices.SortFunc(queuedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return strings.Compare(d1.Env, d2.Env)
				})

				slices.SortFunc(tc.ExpectedDeployments, func(d1 *QueuedDeployment, d2 *QueuedDeployment) int {
					return strings.Compare(d1.Env, d2.Env)
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
		Env                 string
	}{
		{
			Name: "Read all queued versions on environment",
			Env:  envName,
			Deployments: []QueuedDeployment{
				{
					Env:     envName,
					App:     "foo",
					Version: version(0),
				},
				{
					Env:     envName,
					App:     "bar",
					Version: version(0),
				},
				{
					EslVersion: 2,
					Env:        envName,
					App:        "bar",
					Version:    version(1),
				},
				{
					Env:     "fakeEnv",
					App:     "foo",
					Version: version(0),
				},
			},
			ExpectedDeployments: []*QueuedDeployment{
				{
					EslVersion: 1,
					Env:        envName,
					App:        "foo",
					Version:    version(0),
				},
				{
					EslVersion: 2,
					Env:        envName,
					App:        "bar",
					Version:    version(1),
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
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.Version, false)
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
		Env          string
		LockID       string
		Message      string
		TeamName     string
		AuthorName   string
		AuthorEmail  string
		CiLink       string
		ExpectedLock *TeamLock
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
			ExpectedLock: &TeamLock{
				Env:        "dev",
				LockID:     "dev-team-lock",
				EslVersion: 1,
				Deleted:    false,
				Team:       "my-team",
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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
				if diff := cmp.Diff(tc.ExpectedLock, &target, cmpopts.IgnoreFields(TeamLock{}, "Created")); diff != "" {
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
		Env           string
		LockID        string
		Message       string
		TeamName      string
		AuthorName    string
		AuthorEmail   string
		ExpectedLocks []TeamLock
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
			ExpectedLocks: []TeamLock{
				{ //Sort DESC
					Env:        "dev",
					Team:       "my-team",
					LockID:     "dev-lock",
					EslVersion: 2,
					Deleted:    true,
					Metadata: LockMetadata{
						Message:        "My lock on dev for my-team",
						CreatedByName:  "myself",
						CreatedByEmail: "myself@example.com",
					},
				},
				{
					Env:        "dev",
					LockID:     "dev-lock",
					Team:       "my-team",
					EslVersion: 1,
					Deleted:    false,
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
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *envLock))
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

				if diff := cmp.Diff(&tc.ExpectedLocks, &actual, cmpopts.IgnoreFields(TeamLock{}, "Created")); diff != "" {
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
				Created:       time.Now(),
				ReleaseNumber: 1,
				App:           "app",
				Manifests: DBReleaseManifests{
					Manifests: map[string]string{"development": "development"},
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

				errDelete := dbHandler.DBDeleteFromReleases(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber)
				if errDelete != nil {
					t.Fatalf("error: %v\n", errDelete)
				}

				allReleases, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, tc.toInsert.App)
				if err != nil {
					return err
				}
				if len(allReleases) != 0 {
					t.Fatalf("number of team locks mismatch (-want, +got):\n%d", len(allReleases))
				}

				latestRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber, true)
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
		EnvironmentName   string
		EnvironmentConfig config.EnvironmentConfig
		Applications      []string
	}
	type TestCase struct {
		Name          string
		EnvsToWrite   []EnvAndConfig
		EnvToQuery    string
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
				t.Fatalf("the received environment entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s\n", tc.ExpectedEntry, envEntry, diff)
			}
		})
	}
}

func TestReadEnvironmentBatch(t *testing.T) {
	type EnvAndConfig struct {
		EnvironmentName   string
		EnvironmentConfig config.EnvironmentConfig
		Applications      []string
	}
	type TestCase struct {
		Name         string
		EnvsToWrite  []EnvAndConfig
		EnvsToQuery  []string
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
			EnvsToQuery: []string{"development", "staging"},
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
			EnvsToQuery: []string{"development", "staging"},
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
				t.Fatalf("the received environment entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s\n", tc.ExpectedEnvs, environments, diff)
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
		"metadata": map[string]string{
			"authorEmail": authorEmail,
			"authorName":  authorName,
		},
	})
	expectedJsonWithoutMetadata, _ := json.Marshal(map[string]interface{}{
		"env":     envName,
		"app":     appName,
		"lockId":  lockId,
		"message": message,
		"metadata": map[string]string{
			"authorEmail": "",
			"authorName":  "",
		},
	})

	tcs := []struct {
		Name          string
		EventType     EventType
		EventData     *map[string]string
		EventMetadata ESLMetadata
		ExpectedEsl   EslEventRow
	}{
		{
			Name:      "Write and read",
			EventType: evtType,
			EventData: &map[string]string{
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
			EventData: &map[string]string{
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
			err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
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
	const envName = "dev"
	const appName = "my-app"
	const lockId = "ui-v2-ke1up"
	const message = "test"
	const authorName = "testauthor"
	const authorEmail = "testemail@example.com"

	tcs := []struct {
		Name   string
		Events []EslEventRow
		Limit  int
	}{
		{
			Name: "Write and read once",
			Events: []EslEventRow{
				{
					EventType:  EvtCreateApplicationVersion,
					EventJson:  string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 1,
				},
			},
			Limit: 1,
		},
		{
			Name: "Write and read multiple",
			Events: []EslEventRow{
				{
					EventType:  EvtCreateApplicationVersion,
					EventJson:  string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 1,
				},
				{
					EventType:  EvtCreateEnvironmentApplicationLock,
					EventJson:  string(`{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 2,
				},
				{
					EventType:  EvtCreateEnvironment,
					EventJson:  string(`{"env":"dev3","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 3,
				},
			},
			Limit: 3,
		},
		{
			Name: "More than limit",
			Events: []EslEventRow{
				{
					EventType:  EvtCreateApplicationVersion,
					EventJson:  string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 1,
				},
				{
					EventType:  EvtCreateEnvironmentGroupLock,
					EventJson:  string(`{"env":"dev2","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 2,
				},
				{
					EventType:  EvtCreateEnvironment,
					EventJson:  string(`{"env":"dev3","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 3,
				},
			},
			Limit: 2,
		},
		{
			Name: "Less than limit",
			Events: []EslEventRow{
				{
					EventType:  EvtCreateApplicationVersion,
					EventJson:  string(`{"env":"dev","app":"my-app","lockId":"ui-v2-ke1up","message":"test","metadata":{"authorEmail":"testemail@example.com","authorName":"testauthor"}}`),
					Created:    time.Now(),
					EslVersion: 1,
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
			err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				for _, event := range tc.Events {
					err := dbHandler.DBWriteFailedEslEvent(ctx, transaction, &event)
					if err != nil {
						return err
					}
				}

				actualEvents, err := dbHandler.DBReadLastFailedEslEvents(ctx, transaction, tc.Limit)
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
		AllEnvsToWrite []string
		ExpectedEntry  []string
	}
	testCases := []TestCase{
		{
			Name:           "create entry with one environment entry only",
			AllEnvsToWrite: []string{"development"},
			ExpectedEntry:  []string{"development"},
		},
		{
			Name:           "create entries with increasing length",
			AllEnvsToWrite: []string{"development", "production", "staging"},
			ExpectedEntry:  []string{"development", "production", "staging"},
		},
		{
			Name:           "ensure that environments are sorted",
			AllEnvsToWrite: []string{"staging", "development", "production"},
			ExpectedEntry:  []string{"development", "production", "staging"},
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

			allEnvsEntry, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*[]string, error) {
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
				t.Fatalf("the received entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s\n", tc.ExpectedEntry, allEnvsEntry, diff)
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
				t.Fatalf("the received entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s\n", tc.ExpectedEntry, *allAppsEntry, diff)
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
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Retrieved release has ordered environments",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1", "staging": "manfest2", "production": "manfest2"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1", "staging": "manfest2", "production": "manfest2"}},
					Environments:  []string{"dev", "production", "staging"},
				},
			},
		},
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 20,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
				},
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
				{
					ReleaseNumber: 20,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest4"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 20,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Retrieve no releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1`",
			Expected: nil,
		},
		{
			Name: "Different Releases with different eslVersions",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
				},
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Prepublish Release should not be retrieved",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
				},
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: false,
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Prepublish Release should be retrieved",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
				},
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: true,
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
					Environments:  []string{"dev"},
				},
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
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
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Environments:  []string{"dev"},
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
				},
			},
		},
		{
			Name: "Retrieve no releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
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
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 11,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{11},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 11,
					App:           "app1",
					Environments:  []string{"dev"},
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
				},
			},
		},
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 11,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10, 11},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Environments:  []string{"dev"},
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
				},
				{
					ReleaseNumber: 11,
					App:           "app1",
					Environments:  []string{"dev"},
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
				},
			},
		},
		{
			Name: "Retrieve latest esl version only",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1", "staging": "manifest2"}},
				},
			},
			AppName:  "app1",
			Versions: []uint64{10},
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					App:           "app1",
					Environments:  []string{"dev", "staging"},
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
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
		Expected map[string][]int64
	}{
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 1,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			Expected: map[string][]int64{
				"app1": {1, 2},
				"app2": {1, 2},
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
					allReleases[release.App] = []int64{int64(release.ReleaseNumber)}
				} else {
					allReleases[release.App] = append(allReleases[release.App], int64(release.ReleaseNumber))
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
		Expected map[string]map[uint64][]string
	}{
		{
			Name: "Retrieve no manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{}},
				},
			},
			Expected: map[string]map[uint64][]string{
				"app1": {1: {}},
			},
		},
		{
			Name: "Retrieve all manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1", "staging": "manifest2"}},
				},
				{
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					ReleaseNumber: 1,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			Expected: map[string]map[uint64][]string{
				"app1": {
					1: {"dev", "staging"},
					2: {"dev"},
				},
				"app2": {
					1: {"dev"},
				},
			},
		},
		{
			Name: "Retrieve only latest manifests",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1", "staging": "manifest2"}},
				},
				{
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			Expected: map[string]map[uint64][]string{
				"app1": {
					1: {"dev"},
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
				manifests, err := dbHandler.DBSelectAllManifestsForAllReleases(ctx, transaction)
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

func TestReadWriteOverviewCache(t *testing.T) {
	var upstreamLatest = true
	var dev = "dev"
	type TestCase struct {
		Name      string
		Overviews []*api.GetOverviewResponse
	}
	tcs := []TestCase{
		{
			Name: "Read and write",
			Overviews: []*api.GetOverviewResponse{
				&api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "dev",
							Environments: []*api.Environment{
								{
									Name: "development",
									Config: &api.EnvironmentConfig{
										Upstream: &api.EnvironmentConfig_Upstream{
											Latest: &upstreamLatest,
										},
										Argocd:           &api.EnvironmentConfig_ArgoCD{},
										EnvironmentGroup: &dev,
									},
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "test",
							Team: "team-123",
						},
					},
					GitRevision: "0",
				},
			},
		},
		{
			Name: "Read and write multiple",
			Overviews: []*api.GetOverviewResponse{
				&api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "dev",
							Environments: []*api.Environment{
								{
									Name: "development",
									Config: &api.EnvironmentConfig{
										Upstream: &api.EnvironmentConfig_Upstream{
											Latest: &upstreamLatest,
										},
										Argocd:           &api.EnvironmentConfig_ArgoCD{},
										EnvironmentGroup: &dev,
									},
									Priority: api.Priority_YOLO,
								},
							},
							Priority: api.Priority_YOLO,
						},
					},
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "test",
							Team: "team-123",
						},
					},
					GitRevision: "0",
				},
				&api.GetOverviewResponse{
					EnvironmentGroups: []*api.EnvironmentGroup{
						{
							EnvironmentGroupName: "test",
							Environments: []*api.Environment{
								{
									Name: "testing",
									Config: &api.EnvironmentConfig{
										Upstream: &api.EnvironmentConfig_Upstream{
											Latest: &upstreamLatest,
										},
										Argocd:           &api.EnvironmentConfig_ArgoCD{},
										EnvironmentGroup: &dev,
									},
									Priority: api.Priority_CANARY,
								},
							},
							Priority: api.Priority_CANARY,
						},
					},
					LightweightApps: []*api.OverviewApplication{
						{
							Name: "test2",
							Team: "team-123",
						},
					},
					GitRevision: "0",
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
				for _, overview := range tc.Overviews {
					err := dbHandler.WriteOverviewCache(ctx, transaction, overview)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}

				result, err := dbHandler.ReadLatestOverviewCache(ctx, transaction)
				if err != nil {
					return fmt.Errorf("error while selecting release, error: %w", err)
				}

				opts := getOverviewIgnoredTypes()
				if diff := cmp.Diff(tc.Overviews[len(tc.Overviews)-1], result, opts); diff != "" {
					return fmt.Errorf("overview cache ESL ID mismatch (-want +got):\n%s", diff)
				}

				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}
		})
	}
}

func TestFindEnvAppsFromReleases(t *testing.T) {
	type TestCase struct {
		Name             string
		Releases         []DBReleaseWithMetaData
		ExpectedEnvsApps map[string][]string
	}
	tcs := []TestCase{
		{
			Name: "Simple test: several releases",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
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
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app2",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env1": "testmanifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app3",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env1": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[string][]string{
				"env1": {"app1", "app2", "app3"},
				"env2": {"app1"},
				"env3": {"app1", "app2"},
			},
		},
		{
			Name: "Several Releases for one app",
			Releases: []DBReleaseWithMetaData{
				{
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
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
					ReleaseNumber: 11,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env1": "testmanifest",
							"env3": "test",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumber: 12,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env4": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[string][]string{
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
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env1": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env2": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
				{
					ReleaseNumber: 10,
					Created:       time.Now(),
					App:           "app1",
					Manifests: DBReleaseManifests{
						Manifests: map[string]string{
							"env3": "testmanifest",
						},
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor: "testauthor",
					},
				},
			},
			ExpectedEnvsApps: map[string][]string{
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
	dir, err := testutil.CreateMigrationsPath(2)
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)
	cfg := DBConfig{
		MigrationsPath: dir,
		DriverName:     "sqlite3",
		DbHost:         tmpDir,
	}

	migErr := RunDBMigrations(ctx, cfg)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := Connect(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}

func SetupRepositoryTestWithDB(t *testing.T) *DBHandler {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(2)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &DBConfig{
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
		return nil
	}
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("error waiting %v", err)
		return nil
	}
	t.Logf("test created dir: %s", localDir)

	dbConfig.DbHost = dir

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
					ReleaseNumber: 1,
					App:           "appNoEnv",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName: "appNoEnv",
			Expected: []*DBReleaseWithMetaData{
				{
					ReleaseNumber: 1,
					App:           "appNoEnv",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{},
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
		release.ReleaseNumber,
		release.App,
		manifestJson,
		metadataJson,
		environmentStr,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert release for app '%s' and version '%v' into DB. Error: %w\n",
			release.App,
			release.ReleaseNumber,
			err)
	}

	return nil
}
