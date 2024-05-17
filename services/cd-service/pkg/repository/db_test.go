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
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/google/go-cmp/cmp"
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
			cfg := db.DBConfig{
				DriverName: "sqlite3",
				DbHost:     dir,
			}
			db, err := db.Connect(cfg)
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
		expectedData  *db.AllApplicationsGo
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
			expectedData: &db.AllApplicationsGo{
				Version: 1,
				Created: time.Unix(1713218400, 0).UTC(),
				AllApplicationsJson: db.AllApplicationsJson{
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
			cfg := db.DBConfig{
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

			migErr := db.RunDBMigrations(cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := db.Connect(cfg)
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
				Environment:                 envProduction,
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
			cfg := db.DBConfig{
				DriverName:     "sqlite3",
				DbHost:         dbDir,
				MigrationsPath: "/kp/database/migrations",
			}
			migErr := db.RunDBMigrations(cfg)
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			db, err := db.Connect(cfg)
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

			actualQuery := db.SqliteToPostgresQuery(tc.inputQuery)
			if diff := cmp.Diff(tc.expectedQuery, actualQuery); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	tcs := []struct {
		Name                string
		inputHandler        *db.DBHandler
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
			inputHandler: &db.DBHandler{
				WriteEslOnly: true,
			},
			expectedEslTable:    true,
			expectedOtherTables: false,
		},
		{
			Name: "other tables",
			inputHandler: &db.DBHandler{
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
