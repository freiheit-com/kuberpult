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

package integration_tests

import (
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/db"
)

func deleteSchemaMigrationsTable(cfg db.DBConfig) error {
	db, err := db.GetDBConnection(cfg)
	if err != nil {
		return err
	}
	_, err = db.Exec("DROP TABLE schema_migrations")
	if err != nil {
		return err
	}
	return nil
}
func TestMigrations(t *testing.T) {
	dbConfig := db.DBConfig{
		DbHost:         "localhost",
		DbPort:         "5432",
		DbUser:         "postgres",
		DbPassword:     "mypassword",
		DbName:         "kuberpult",
		DriverName:     "postgres",
		MigrationsPath: "../../database/migrations/postgres",
		SSLMode:        "disable",
	}
	dbHandler, err := db.Connect(dbConfig)
	if err != nil {
		t.Fatalf("Error establishing DB connection: %v", err)
	}
	pErr := dbHandler.DB.Ping()
	if pErr != nil {
		t.Fatalf("Error pinging database: %v", err)
	}
	if err := deleteSchemaMigrationsTable(dbConfig); err != nil {
		t.Fatalf("Failed to delete schema migrations table: %v", err)
	}
	testCases := []struct {
		name string
	}{
		{
			name: "Running migrations multiple times",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run migrations for the first time
			if err := db.RunDBMigrations(dbConfig); err != nil {
				t.Errorf("Error running migrations: %v", err)
			}
			// Delete schema migrations
			if err := deleteSchemaMigrationsTable(dbConfig); err != nil {
				t.Fatalf("Failed to delete schema migrations table: %v", err)
			}
			// Run migrations again
			if err := db.RunDBMigrations(dbConfig); err != nil {
				t.Errorf("Error running migrations: %v", err)
			}
		})
	}

}
