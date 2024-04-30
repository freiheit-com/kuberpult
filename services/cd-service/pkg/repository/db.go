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

type DBConfig struct {
	DbUser         string
	DbHost         string
	DbPort         string
	DbName         string
	DriverName     string
	DbPassword     string
	MigrationsPath string
}

type DBHandler struct {
	DbName         string
	DriverName     string
	MigrationsPath string
	DB             *sql.DB
	DBDriver       *database.Driver
}

func Connect(cfg DBConfig) (*DBHandler, error) {
	db, driver, err := GetConnectionAndDriver(cfg)

	if err != nil {
		return nil, err
	}
	return &DBHandler{
		DbName:         cfg.DbName,
		DriverName:     cfg.DriverName,
		MigrationsPath: cfg.MigrationsPath,
		DB:             db,
		DBDriver:       &driver,
	}, nil
}

func GetDBConnection(cfg DBConfig) (*sql.DB, error) {
	if cfg.DriverName == "postgres" {
		dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=disable",
			cfg.DbHost, cfg.DbUser, cfg.DbPassword, cfg.DbPort, cfg.DbName)

		dbPool, err := sql.Open(cfg.DriverName, dbURI)
		if err != nil {
			return nil, fmt.Errorf("sql.Open: %w", err)
		}
		dbPool.SetConnMaxLifetime(5 * time.Minute)
		return dbPool, nil
	} else if cfg.DriverName == "sqlite3" {
		return sql.Open("sqlite3", path.Join(cfg.DbHost, "db.sqlite"))
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", cfg.DriverName)
}

func GetConnectionAndDriver(cfg DBConfig) (*sql.DB, database.Driver, error) {
	db, err := GetDBConnection(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg.DriverName == "postgres" {
		driver, err := psql.WithInstance(db, &psql.Config{
			DatabaseName:          cfg.DbName,
			MigrationsTable:       "",
			MigrationsTableQuoted: false,
			MultiStatementEnabled: false,
			MultiStatementMaxSize: 0,
			SchemaName:            "",
			StatementTimeout:      time.Second * 10,
		})
		return db, driver, err
	} else if cfg.DriverName == "sqlite3" {
		driver, err := sqlite.WithInstance(db, &sqlite.Config{
			DatabaseName:    "",
			MigrationsTable: "",
			NoTxWrap:        false,
		})
		return db, driver, err
	}
	return nil, nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", cfg.DriverName)
}

func (d *DBHandler) getMigrationHandler() (*migrate.Migrate, error) {
	if d.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+d.MigrationsPath, d.DbName, *d.DBDriver)
	} else if d.DriverName == "sqlite3" {
		return migrate.NewWithDatabaseInstance("file://"+d.MigrationsPath, "", *d.DBDriver) //FIX ME
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", d.DriverName)
}

func RunDBMigrations(cfg DBConfig) error {
	d, err := Connect(cfg)
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer d.DB.Close()

	m, err := d.getMigrationHandler()

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

func (d *DBHandler) insertDummyRow() (sql.Result, error) {

	stmt, err := d.getDummyPreparedStatement(d.DB)

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

	result, err := d.insertDummyRow()

	if err != nil {
		return nil, fmt.Errorf("Error inserting row. Error: %w\n", err)
	}

	return result, nil
}

func (d *DBHandler) RetrieveDatabaseInformation() (map[int]DummyDbRow, error) {
	rows, err := d.DB.Query("SELECT * FROM dummy_table;")

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
