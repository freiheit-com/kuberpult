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
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"math/rand"
	"path"
	"reflect"
	"slices"
	"strings"
	"time"
)

func GetDBConnection(dbFolderLocation string) (*sql.DB, error) {
	return sql.Open("sqlite3", path.Join(dbFolderLocation, "db.sqlite"))
}

func GetDBConnectionOrPanic(dbFolderLocation string) *sql.DB {
	db, err := GetDBConnection(dbFolderLocation)
	if err != nil {
		panic(err)
	}
	return db
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

func RunCustomMigrations(ctx context.Context, dbFolderLocation string, repo Repository) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	db, err := GetDBConnection(dbFolderLocation)
	// TODO SU span
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	l := logger.FromContext(ctx).Sugar()
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			l.Warnf("Could not close db connection: %v", err)
		}
	}(db)

	allAppsDb, err := DBSelectAllApplications(ctx, db)
	if err != nil {
		l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
		allAppsDb = nil
	}

	allAppsRepo, err := repo.State().GetApplicationsFromFile()
	if err != nil {
		return fmt.Errorf("could not get applications to run custom migrations: %v", err)
	}
	var version int64 = 0
	if allAppsDb != nil {
		slices.Sort(allAppsDb.Apps)
		version = allAppsDb.Version
	} else {
		version = 1
	}
	slices.Sort(allAppsRepo)

	if allAppsDb != nil && reflect.DeepEqual(allAppsDb.Apps, allAppsRepo) {
		// nothing to do
		logger.FromContext(ctx).Sugar().Infof("Nothing to do, all apps are equal")
		return nil
	}
	// if there is any difference, we assume the manifest wins over the database state:
	return DBWriteAllApplications(ctx, db, version, allAppsRepo)
}

type AllApplicationsJson struct {
	Apps []string `json:"apps"`
}

type AllApplicationsRow struct {
	version int64
	created time.Time
	data    string
}

type AllApplicationsGo struct {
	Version int64
	Created time.Time
	AllApplicationsJson
}

func DBWriteAllApplications(_ context.Context, db *sql.DB, previousVersion int64, applications []string) error {
	jsonToInsert, _ := json.Marshal(AllApplicationsJson{
		Apps: applications,
	})
	_, err := db.Exec(
		"INSERT INTO all_apps (version , created , json)  VALUES (?, ?, ?);",
		previousVersion+1,
		time.Now(),
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	}
	return nil
}

// DBSelectAllApplications returns (nil, nil) if there are no rows
func DBSelectAllApplications(ctx context.Context, db *sql.DB) (*AllApplicationsGo, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer span.Finish()
	query := "SELECT version, created, json FROM all_apps ORDER BY version DESC LIMIT 1;"
	span.SetTag("sqlQuery", query)
	rows := db.QueryRowContext(ctx, query)
	result := AllApplicationsRow{}

	err := rows.Scan(&result.version, &result.created, &result.data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning row from DB. Error: %w\n", err)
	}

	var resultGo = AllApplicationsGo{}
	var resultJson = AllApplicationsJson{}
	err = json.Unmarshal(([]byte)(result.data), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("Error during json unmarshal. Error: %w\n", err)
	}
	resultGo.Version = result.version
	resultGo.Created = result.created
	resultGo.Apps = resultJson.Apps
	return &resultGo, nil
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
