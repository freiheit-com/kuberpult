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
	"strconv"
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
		expectedData  *AllApplicationsGo
	}{
		{
			Name: "Simple migration",
			migrationFile: `
CREATE TABLE IF NOT EXISTS all_apps
(
    version BIGINT,
    created TIMESTAMP,
    json VARCHAR(255),
    PRIMARY KEY(version)
);

INSERT INTO all_apps (version , created , json)  VALUES (0, 	'1713218400', 'First Message');
INSERT INTO all_apps (version , created , json)  VALUES (1, 	'1713218400', '{"apps":["my-test-app"]}');`,
			expectedData: &AllApplicationsGo{
				Version: 1,
				Created: time.Unix(1713218400, 0).UTC(),
				AllApplicationsJson: AllApplicationsJson{
					Apps: []string{"my-test-app"},
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
		releaseNumbers := []int64{}
		for _, r := range releases {
			dbRelease := DBReleaseWithMetaData{
				EslVersion:    InitialEslVersion,
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
				Deleted: false,
			}
			err := dbHandler.DBInsertRelease(ctx, transaction, dbRelease, InitialEslVersion-1)
			if err != nil {
				return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
			}
			releaseNumbers = append(releaseNumbers, int64(r.Version))
		}
		err := dbHandler.DBInsertAllReleases(ctx, transaction, app, releaseNumbers, InitialEslVersion-1)
		if err != nil {
			return fmt.Errorf("error writing all_releases to DB for app %s: %v", app, err)
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
					EslVersion:    1,
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
					EslVersion:    1,
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
					EslVersion:  2,
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
				if diff := cmp.Diff(tc.expectedAllApps, allApps.Apps); diff != "" {
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
				EslVersion:    2,
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
				EslVersion:    2,
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

				err = dbHandler.DBWriteDeployment(ctx, transaction, Deployment{
					App:           tc.App,
					Env:           tc.Env,
					Version:       tc.VersionToDeploy,
					TransformerID: 0,
				}, 1, false)
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
				for _, d := range tc.data {
					err := dbHandler.DBUpdateAllDeploymentsForApp(ctx, transaction, tc.AppName, d.EnvName, int64(d.Version))
					if err != nil {
						t.Fatalf("Error updating all deployments: %v\n", err)
					}
				}
				result, err := dbHandler.DBSelectAllDeploymentsForApp(ctx, transaction, tc.AppName)
				if err != nil {
					t.Fatalf("Error reading from all deployments: %v\n", err)
				}
				if diff := cmp.Diff(tc.expected, result.Deployments); diff != "" {
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
		expected DBReleaseWithMetaData
	}{
		{
			Name: "Delete Release from database",
			toInsert: DBReleaseWithMetaData{
				EslVersion:    InitialEslVersion,
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
				Deleted: false,
			},
			expected: DBReleaseWithMetaData{
				EslVersion:    InitialEslVersion + 1,
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
				Deleted:      true,
				Environments: []string{"development"},
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
				err2 := dbHandler.DBInsertRelease(ctx, transaction, tc.toInsert, tc.toInsert.EslVersion-1)
				if err2 != nil {
					return err2
				}
				err2 = dbHandler.DBInsertAllReleases(ctx, transaction, tc.toInsert.App, []int64{int64(tc.toInsert.ReleaseNumber)}, tc.toInsert.EslVersion-1)
				if err2 != nil {
					return err2
				}

				errDelete := dbHandler.DBDeleteFromReleases(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber)
				if errDelete != nil {
					t.Fatalf("error: %v\n", errDelete)
				}

				errDelete2 := dbHandler.DBDeleteReleaseFromAllReleases(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber)
				if errDelete2 != nil {
					return errDelete2
				}

				allReleases, err := dbHandler.DBSelectAllReleasesOfApp(ctx, transaction, tc.toInsert.App)
				if err != nil {
					return err
				}
				if len(allReleases.Metadata.Releases) != 0 {
					t.Fatalf("number of team locks mismatch (-want, +got):\n%d", len(allReleases.Metadata.Releases))
				}

				latestRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber, true)
				if err != nil {
					return err
				}
				if !latestRelease.Deleted {
					t.Fatalf("Release has not beed deleted\n")
				}

				if diff := cmp.Diff(&tc.expected, latestRelease, cmpopts.IgnoreFields(DBReleaseWithMetaData{}, "Created")); diff != "" {
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
				Version:      1,
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
				Version:      1,
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
				Version: 2,
				Name:    "development",
				Config:  testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("development-group")),
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
				Version: 3,
				Name:    "development",
				Config:  testutil.MakeEnvConfigLatestWithGroup(nil, conversion.FromString("another-development-group")),
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
				Version: 1,
				Name:    "staging",
				Config:  testutil.MakeEnvConfigUpstream("development", nil),
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
		AllEnvsToWrite [][]string
		ExpectedEntry  *DBAllEnvironments
	}
	testCases := []TestCase{
		{
			Name: "create entry with one environment entry only",
			AllEnvsToWrite: [][]string{
				{"development"},
			},
			ExpectedEntry: &DBAllEnvironments{
				Version:      1,
				Environments: []string{"development"},
			},
		},
		{
			Name: "create entries with increasing length",
			AllEnvsToWrite: [][]string{
				{"development"},
				{"development", "staging"},
				{"development", "staging", "production"},
			},
			ExpectedEntry: &DBAllEnvironments{
				Version:      3,
				Environments: []string{"development", "staging", "production"},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)

			for _, allEnvs := range tc.AllEnvsToWrite {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := dbHandler.DBWriteAllEnvironments(ctx, transaction, allEnvs)
					if err != nil {
						return fmt.Errorf("error while writing environment, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing all environments %v to the database, error: %v", allEnvs, err)
				}
			}

			allEnvsEntry, err := WithTransactionT(dbHandler, ctx, DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*DBAllEnvironments, error) {
				allEnvsEntry, err := dbHandler.DBSelectAllEnvironments(ctx, transaction)
				if err != nil {
					return nil, fmt.Errorf("error while selecting environment entry, error: %w", err)
				}
				return allEnvsEntry, nil
			})

			if err != nil {
				t.Fatalf("error while running the transaction for selecting the target all environment, error: %v", err)
			}
			if diff := cmp.Diff(allEnvsEntry, tc.ExpectedEntry, cmpopts.IgnoreFields(DBAllEnvironments{}, "Created")); diff != "" {
				t.Fatalf("the received entry is different from expected\n  expected: %v\n  received: %v\n  diff: %s\n", tc.ExpectedEntry, allEnvsEntry, diff)
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
					EslVersion:    1,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    1,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Retrieve deleted release",
			Releases: []DBReleaseWithMetaData{
				{
					EslVersion:    1,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 10,
					App:           "app1",
					Deleted:       true,
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    2,
					ReleaseNumber: 10,
					Deleted:       true,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Retrieve multiple releases",
			Releases: []DBReleaseWithMetaData{
				{
					EslVersion:    2,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 20,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
				},
				{
					EslVersion:    1,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 20,
					App:           "app2",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest4"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    2,
					ReleaseNumber: 20,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 10,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
					Environments:  []string{"dev"},
				},
			},
		},
		{
			Name: "Retrieve no releases",
			Releases: []DBReleaseWithMetaData{
				{
					EslVersion:    2,
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
					EslVersion:    1,
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
				},
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName: "app1",
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    1,
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
					EslVersion:    1,
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
				},
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: false,
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    1,
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
					EslVersion:    1,
					ReleaseNumber: 1,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest1"}},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
				},
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
				},
			},
			AppName:              "app1",
			RetrievePrepublishes: true,
			Expected: []*DBReleaseWithMetaData{
				{
					EslVersion:    1,
					ReleaseNumber: 3,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest3"}},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    2,
					ReleaseNumber: 2,
					App:           "app1",
					Manifests:     DBReleaseManifests{Manifests: map[string]string{"dev": "manifest2"}},
					Metadata:      DBReleaseMetaData{IsPrepublish: true},
					Environments:  []string{"dev"},
				},
				{
					EslVersion:    1,
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
					err := dbHandler.DBInsertRelease(ctx, transaction, release, release.EslVersion-1)
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

func TestReadWriteOverviewCache(t *testing.T) {
	var upstreamLatest = true
	var dev = "dev"
	type TestCase struct {
		Name      string
		Overviews []*api.GetOverviewResponse
	}
	//TODO: This test suite has some commented out sections. These should be resolved in Ref: SRX-9PBRYS.
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
