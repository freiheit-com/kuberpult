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

Copyright 2023 freiheit.com*/

package repository

import (
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
			handler := DBHandler{
				DriverName: "sqlite3",
				DbHost:     dir,
			}
			connection, err := handler.GetDBConnection()
			if err != nil {
				t.Fatalf("Error establishing DB connection. Error: %v\n", err)
			}
			defer connection.Close()
			pingErr := connection.Ping()
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
		expectedData  []DummyDbRow
	}{
		{
			Name: "Simple migration",
			migrationFile: `CREATE TABLE IF NOT EXISTS dummy_table
(   id BIGINT,
    created TIMESTAMP,
    data VARCHAR(255),
    PRIMARY KEY(id)
);

INSERT INTO dummy_table (id , created , data)  VALUES (0, 	'1713218400', 'First Message');
INSERT INTO dummy_table (id , created , data)  VALUES (1, 	'1713218400', 'Second Message');`,
			expectedData: []DummyDbRow{
				{
					id:   0,
					date: []byte("2024-04-15T22:00:00Z"),
					data: "First Message",
				},
				{
					id:   1,
					date: []byte("2024-04-15T22:00:00Z"),
					data: "Second Message",
				},
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			dbDir := t.TempDir()
			handler := DBHandler{
				DriverName: "sqlite3",
				DbHost:     dbDir,
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

			migErr := handler.RunDBMigrations()
			if migErr != nil {
				t.Fatalf("Error running migration script. Error: %v\n", migErr)
			}

			m, err := handler.RetrieveDatabaseInformation()
			if err != nil {
				t.Fatalf("Error querying dabatabse. Error: %v\n", err)
			}
			//parse the DB data
			for _, r := range tc.expectedData {
				if val, ok := m[r.id]; !ok || !val.Equal(r) { //Not in map or in map but not Equal
					t.Fatalf("Expected data not present in database. Missing: [%d, %s, %s]", r.id, string(r.date), r.data)
				}
			}
		})
	}
}
