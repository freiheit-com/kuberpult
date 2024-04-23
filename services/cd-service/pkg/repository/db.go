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
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang-migrate/migrate/v4"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"math/rand"
	"path"
	"strings"
	"time"
)

func GetDBConnection(dbFolderLocation string) (*sql.DB, error) {
	return sql.Open("sqlite3", path.Join(dbFolderLocation, "db.sqlite"))
}
func GetRemoteConnection() (*sql.DB, error) {
	var (
		dbUser    = "postgres"     // e.g. 'my-db-user'
		dbTCPHost = "127.0.0.1"    // e.g. '127.0.0.1' ('172.17.0.1' if deployed to GAE Flex)
		dbPort    = "5432"         // e.g. '5432'
		dbName    = "test-service" // e.g. 'my-database'
	)

	dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s",
		dbTCPHost, dbUser, "", dbPort, dbName)

	dbPool, err := sql.Open("postgres", dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	return dbPool, nil
}
func RunDBMigrations(dbFolderLocation string) error {
	db, err := GetDBConnection(dbFolderLocation)
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer db.Close()

	driver, err := sqlite.WithInstance(db, &sqlite.Config{
		DatabaseName:    "",
		MigrationsTable: "",
		NoTxWrap:        false,
	})
	if err != nil {
		return fmt.Errorf("Error creating DB driver. Error: %w\n", err)
	}

	migrationsSrc, err := (&file.File{PartialDriver: iofs.PartialDriver{}}).Open(path.Join(dbFolderLocation, "migrations"))
	if err != nil {
		return fmt.Errorf("Error opening DB migrations. Error: %w\n", err)
	}

	m, err := migrate.NewWithInstance("file", migrationsSrc, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("Error creating migration instance. Error: %w\n", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("Error running DB migrations. Error: %w\n", err)
		}
	}
	return nil
}

type DummyDbRow struct {
	id   int
	date []byte
	data string
}

func (r *DummyDbRow) Equal(target DummyDbRow) bool {
	return r.data == target.data && r.id == target.id && string(r.date) == string(target.date)
}

func RetrieveDatabaseInformation(databaseLocation string) (map[int]DummyDbRow, error) {
	db, err := GetDBConnection(databaseLocation)

	if err != nil {
		return nil, fmt.Errorf("Error creating DB connection. Error: %w\n", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM dummy_table;")

	if err != nil {
		return nil, fmt.Errorf("Error querying the database. Error: %w\n", err)
	}
	m := map[int]DummyDbRow{}
	for rows.Next() {
		r := DummyDbRow{
			id:   0,
			date: []byte{},
			data: "",
		}
		err := rows.Scan(&r.id, &r.date, &r.data)
		if err != nil {
			return nil, fmt.Errorf("Error retrieving information from database. Error: %w\n", err)
		}
		m[r.id] = r
	}

	return m, nil
}

func InsertDatabaseInformation(databaseLocation string, message string) (sql.Result, error) {
	db, err := GetDBConnection(databaseLocation)
	if err != nil {
		return nil, fmt.Errorf("Error creating DB connection. Error: %w\n", err)
	}
	defer db.Close()

	result, err := db.Exec("INSERT INTO dummy_table (id , created , data)  VALUES (?, ?, ?);", rand.Intn(9999), time.Now(), message)

	if err != nil {
		return nil, fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	}

	return result, nil
}

func getHeader(totalWidth int) string {
	return "+" + strings.Repeat("-", totalWidth-2) + "+"
}

func PrintQuery(queryResult map[int]DummyDbRow) error {
	fmt.Println(getHeader(80))
	for _, val := range queryResult {
		fmt.Printf("| %-4d %50s %20s |\n", val.id, string(val.date), val.data)
	}
	fmt.Println(getHeader(80))
	return nil
}
