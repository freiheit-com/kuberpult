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
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"
	"time"
)

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
			t.Parallel()
			dir := t.TempDir()
			cfg := DBConfig{
				DriverName: "sqlite3",
				DbHost:     dir,
			}
			db, err := Connect(cfg)
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

			migErr := RunDBMigrations(cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := Connect(cfg)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}
			tx, err := db.DB.BeginTx(ctx, nil)
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
	var getAllReleases = /*GetAllReleasesFun*/ func(ctx context.Context, app string) (AllReleases, error) {
		result := AllReleases{
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
		return result, nil
	}
	tcs := []struct {
		Name             string
		expectedReleases []*DBReleaseWithMetaData
	}{
		{
			Name: "Simple migration",
			expectedReleases: []*DBReleaseWithMetaData{
				{
					EslId:         1,
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
				},
				{
					EslId:         1,
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
			err3 := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.RunCustomMigrationReleases(ctx, getAllApps, getAllReleases)
				if err2 != nil {
					return fmt.Errorf("error: %v", err2)
				}
				for i := range tc.expectedReleases {
					expectedRelease := tc.expectedReleases[i]

					release, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, expectedRelease.App, expectedRelease.ReleaseNumber)
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

func TestDeploymentStorage(t *testing.T) {
	tcs := []struct {
		Name       string
		commitHash string
		email      string
		event      event.Deployment
		metadata   event.Metadata
	}{
		{
			Name:       "Simple Deployment event",
			commitHash: "abcdefabcdef",
			email:      "test@email.com",
			event: event.Deployment{
				Environment:                 "production",
				Application:                 "test-app",
				SourceTrainUpstream:         nil,
				SourceTrainEnvironmentGroup: nil,
			},
			metadata: event.Metadata{
				AuthorEmail: "test@email.com",
				Uuid:        "00000000-0000-0000-0000-000000000001",
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
			migErr := RunDBMigrations(cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := Connect(cfg)
			if err != nil {
				t.Fatal("Error establishing DB connection: ", zap.Error(err))
			}

			tx, err := db.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("Error creating transaction. Error: %v\n", err)
			}
			writeDeploymentError := db.DBWriteDeploymentEvent(ctx, tx, tc.metadata.Uuid, tc.commitHash, tc.email, &tc.event)
			if writeDeploymentError != nil {
				t.Fatalf("Error writing event to DB. Error: %v\n", writeDeploymentError)
			}
			err = tx.Commit()
			if err != nil {
				t.Fatalf("Error commiting transaction. Error: %v\n", err)
			}

			m, err := db.DBSelectAllEventsForCommit(ctx, tc.commitHash)
			if err != nil {
				t.Fatalf("Error querying dabatabse. Error: %v\n", err)
			}

			for _, currEvent := range m {
				e, err := event.UnMarshallEvent("deployment", currEvent.EventJson)

				if err != nil {
					t.Fatalf("Error obtaining event from DB. Error: %v\n", err)
				}

				if diff := cmp.Diff(e.EventData, &tc.event); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}

				if diff := cmp.Diff(e.EventMetadata, tc.metadata); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
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
				App:        "app-a",
				Env:        "dev",
				EslVersion: 2,
				Version:    version(7),
			},
		},
		{
			Name:            "with eslVersion == nil",
			App:             "app-b",
			Env:             "prod",
			VersionToDeploy: nil,
			ExpectedDeployment: &Deployment{
				App:        "app-b",
				Env:        "prod",
				EslVersion: 2,
				Version:    nil,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				deployment, err2 := dbHandler.DBSelectAnyDeployment(ctx, transaction)
				if err2 != nil {
					return err2
				}
				if deployment != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *deployment))
				}
				err := dbHandler.DBWriteDeployment(ctx, transaction, Deployment{
					App:     tc.App,
					Env:     tc.Env,
					Version: tc.VersionToDeploy,
				}, 1)
				if err != nil {
					return err
				}

				actual, err := dbHandler.DBSelectDeployment(ctx, transaction, tc.App, tc.Env)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteEnvironmentLock(ctx, transaction, tc.LockID, tc.Env, tc.Message, tc.AuthorName, tc.AuthorEmail)
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

func TestReadWriteEnvironmentLock(t *testing.T) {
	tcs := []struct {
		Name         string
		Env          string
		LockID       string
		Message      string
		AuthorName   string
		AuthorEmail  string
		ExpectedLock *EnvironmentLock
	}{
		{
			Name:        "Simple environment lock",
			Env:         "dev",
			LockID:      "dev-lock",
			Message:     "My lock on dev",
			AuthorName:  "myself",
			AuthorEmail: "myself@example.com",
			ExpectedLock: &EnvironmentLock{
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
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteEnvironmentLock(ctx, transaction, tc.LockID, tc.Env, tc.Message, tc.AuthorName, tc.AuthorEmail)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectAppLock(ctx, transaction, tc.Env, tc.AppName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteApplicationLock(ctx, transaction, tc.LockID, tc.Env, tc.AppName, tc.Message, tc.AuthorName, tc.AuthorEmail)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectEnvironmentLock(ctx, transaction, tc.Env, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteApplicationLock(ctx, transaction, tc.LockID, tc.Env, tc.AppName, tc.Message, tc.AuthorName, tc.AuthorEmail)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				for _, deployments := range tc.Deployments {
					err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, deployments.Env, deployments.App, deployments.Version)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteDeploymentAttempt(ctx, transaction, tc.Env, tc.AppName, tc.Version)
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

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *DBHandler {
	dir, err := testutil.CreateMigrationsPath(2)
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)
	cfg := DBConfig{
		MigrationsPath: dir,
		DriverName:     "sqlite3",
		DbHost:         tmpDir,
	}

	migErr := RunDBMigrations(cfg)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := Connect(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}

func SetupRepositoryTestWithDB(t *testing.T) *DBHandler {
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

	migErr := RunDBMigrations(*dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := Connect(*dbConfig)
	if err != nil {
		t.Fatal(err)
	}
	return dbHandler
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectTeamLock(ctx, transaction, tc.Env, tc.TeamName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteTeamLock(ctx, transaction, tc.LockID, tc.Env, tc.TeamName, tc.Message, tc.AuthorName, tc.AuthorEmail)
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
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				envLock, err2 := dbHandler.DBSelectTeamLock(ctx, transaction, tc.Env, tc.TeamName, tc.LockID)
				if err2 != nil {
					return err2
				}
				if envLock != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *envLock))
				}
				err := dbHandler.DBWriteTeamLock(ctx, transaction, tc.LockID, tc.Env, tc.TeamName, tc.Message, tc.AuthorName, tc.AuthorEmail)
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
			Name: "yeet",
			toInsert: DBReleaseWithMetaData{
				EslId:         InitialEslId,
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
				EslId:         InitialEslId + 1,
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
				Deleted: true,
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := dbHandler.DBInsertRelease(ctx, transaction, tc.toInsert, tc.toInsert.EslId-1)
				if err2 != nil {
					return err2
				}
				err2 = dbHandler.DBInsertAllReleases(ctx, transaction, tc.toInsert.App, []int64{int64(tc.toInsert.ReleaseNumber)}, tc.toInsert.EslId-1)
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

				latestRelease, err := dbHandler.DBSelectReleaseByVersion(ctx, transaction, tc.toInsert.App, tc.toInsert.ReleaseNumber)
				if err != nil {
					return err
				}
				if !latestRelease.Deleted {
					t.Fatalf("Not deleted:\n")
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
