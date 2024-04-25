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
	"math/rand"
	"path"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	psql "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type DBHandler struct {
	DbUser         string
	DbHost         string
	DbPort         string
	DbName         string
	DriverName     string
	DbPassword     string
	MigrationsPath string
}

func (d *DBHandler) GetDBConnection() (*sql.DB, error) {
	if d.DriverName == "postgres" {
		dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=disable",
			d.DbHost, d.DbUser, d.DbPassword, d.DbPort, d.DbName)

		dbPool, err := sql.Open(d.DriverName, dbURI)
		if err != nil {
			return nil, fmt.Errorf("sql.Open: %w", err)
		}
		dbPool.SetConnMaxLifetime(5 * time.Minute)
		return dbPool, nil
	} else if d.DriverName == "sqlite3" {
		return sql.Open("sqlite3", path.Join(d.DbHost, "db.sqlite"))
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", d.DriverName)

}

func (d *DBHandler) getDriver(db *sql.DB) (database.Driver, error) {
	if d.DriverName == "postgres" {
		return psql.WithInstance(db, &psql.Config{
			DatabaseName:          d.DbName,
			MigrationsTable:       "",
			MigrationsTableQuoted: false,
			MultiStatementEnabled: false,
			MultiStatementMaxSize: 0,
			SchemaName:            "",
			StatementTimeout:      time.Second * 10,
		})
	} else if d.DriverName == "sqlite3" {
		return sqlite.WithInstance(db, &sqlite.Config{
			DatabaseName:    "",
			MigrationsTable: "",
			NoTxWrap:        false,
		})
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", d.DriverName)
}

func (d *DBHandler) getMigrationHandler(driver database.Driver) (*migrate.Migrate, error) {
	if d.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+d.MigrationsPath, d.DbName, driver)
	} else if d.DriverName == "sqlite3" {
		return migrate.NewWithDatabaseInstance("file://"+d.MigrationsPath, "", driver) //FIX ME
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", d.DriverName)
}

func (d *DBHandler) RunDBMigrations() error {
	db, err := d.GetDBConnection()
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer db.Close()

	driver, err := d.getDriver(db)

	if err != nil {
		return fmt.Errorf("Error creating DB driver. Error: %w\n", err)
	}

	m, err := d.getMigrationHandler(driver)

	if err != nil {
		return fmt.Errorf("Error creating migration instance. Error: %w\n", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("Error running DB migrations. Error: %w\n", err)
		}
	}
	return nil
}

func (d *DBHandler) getDummyPreparedStatement(db *sql.DB) (*sql.Stmt, error) {
	var statement string
	if d.DriverName == "postgres" { //TODO: Replace ? or $x based on driver
		statement = "INSERT INTO dummy_table (id , created , data)  VALUES ($1, $2, $3);"
	} else if d.DriverName == "sqlite3" {
		statement = "INSERT INTO dummy_table (id , created , data)  VALUES (?, ?, ?);"
	}
	return db.Prepare(statement)
}

func (d *DBHandler) insertDummyRow(db *sql.DB) (sql.Result, error) {

	stmt, err := d.getDummyPreparedStatement(db)

	if err != nil {
		return nil, fmt.Errorf("Error Preparing statement. Error: %w\n", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(rand.Intn(9999), rand.Intn(9999), rand.Intn(9999))

	if err != nil {
		return nil, fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	}

	return result, nil
}

func (d *DBHandler) InsertDatabaseInformation() (sql.Result, error) {
	db, err := d.GetDBConnection()

	if err != nil {
		return nil, fmt.Errorf("Error creating DB connection. Error: %w\n", err)
	}
	defer db.Close()

	result, err := d.insertDummyRow(db)

	if err != nil {
		return nil, fmt.Errorf("Error inserting row. Error: %w\n", err)
	}

	return result, nil
}
func (d *DBHandler) RetrieveDatabaseInformation() (map[int]DummyDbRow, error) {
	db, err := d.GetDBConnection()

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

type DummyDbRow struct {
	id   int
	date []byte
	data string
}

func (r *DummyDbRow) Equal(target DummyDbRow) bool {
	return r.data == target.data && r.id == target.id && string(r.date) == string(target.date)
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
