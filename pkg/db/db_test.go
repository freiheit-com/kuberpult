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

			dir, err := testutil.CreateMigrationsPath()
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

func TestWriteReadDeployment(t *testing.T) {
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

	dir, err := testutil.CreateMigrationsPath()
	if err != nil {
		t.Fatalf("setup error could not detect dir \n%v", err)
		return
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("detected dir: %s - err=%v", dir, err)
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err = dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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
			Name:          "Write and delete",
			Env:           "dev",
			LockID:        "dev-lock",
			Message:       "My lock on dev",
			AuthorName:    "myself",
			AuthorEmail:   "myself@example.com",
			ExpectedLocks: []EnvironmentLock{},
		},
	}

	dir, err := testutil.CreateMigrationsPath()
	if err != nil {
		t.Fatalf("setup error could not detect dir \n%v", err)
		return
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("detected dir: %s - err=%v", dir, err)
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err = dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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

				actual, err := dbHandler.DBSelectEnvLocks(ctx, transaction, tc.Env)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(0, len(actual)); diff != "" {
					t.Fatalf("number of env locks mismatch (-want, +got):\n%s", diff)
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
				Metadata: EnvironmentLockMetadata{
					Message:        "My lock on dev",
					CreatedByName:  "myself",
					CreatedByEmail: "myself@example.com",
				},
			},
		},
	}

	dir, err := testutil.CreateMigrationsPath()
	if err != nil {
		t.Fatalf("setup error could not detect dir \n%v", err)
		return
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("detected dir: %s - err=%v", dir, err)
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)
			err = dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
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

				actual, err := dbHandler.DBSelectEnvLocks(ctx, transaction, tc.Env)
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

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *DBHandler {
	dir, err := testutil.CreateMigrationsPath()
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
