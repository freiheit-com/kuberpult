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
	psql "github.com/golang-migrate/migrate/v4/database/postgres"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

type DBInfo struct {
	DbUser     string
	DbHost     string
	DbPort     string
	DbName     string
	DriverName string
	DbPassword string
}

func (d *DBInfo) GetDBConnection() (*sql.DB, error) {

	dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=disable",
		d.DbHost, d.DbUser, d.DbPassword, d.DbPort, d.DbName)

	dbPool, err := sql.Open(d.DriverName, dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	dbPool.SetConnMaxLifetime(5 * time.Minute)
	return dbPool, nil
}

func (d *DBInfo) RunDBMigrations(migrationsFolder string) error {
	db, err := d.GetDBConnection()
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer db.Close()

	driver, err := psql.WithInstance(db, &psql.Config{
		DatabaseName:          d.DbName,
		MigrationsTable:       "",
		MigrationsTableQuoted: false,
		MultiStatementEnabled: false,
		MultiStatementMaxSize: 0,
		SchemaName:            "",
		StatementTimeout:      time.Second * 10,
	})
	if err != nil {
		return fmt.Errorf("Error creating DB driver. Error: %w\n", err)
	}

	// migrationsSrc, err := (&file.File{PartialDriver: iofs.PartialDriver{}}).Open(migrationsFolder)
	// if err != nil {
	// 	return fmt.Errorf("Error opening DB migrations. Error: %w\n", err)
	// }

	// dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=disable",
	// 	d.DbHost, d.DbUser, d.DbPassword, d.DbPort, d.DbName)

	//m, err := migrate.NewWithInstance("file", migrationsSrc, d.DriverName, driver)
	m, err := migrate.NewWithDatabaseInstance("file:///migrations", d.DbName, driver)
	defer m.Close()
	if err != nil {
		return fmt.Errorf("Error creating migration instance. Error: %w\n", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("Error running DB migrations. Error: %w\n", err)
		}
		fmt.Printf("Migration error: %s\n", err.Error())
	}
	return nil
}

func (d *DBInfo) RetrieveDatabaseInformation(message string) (map[int]DummyDbRow, error) {
	db, err := d.GetDBConnection()

	if err != nil {
		return nil, fmt.Errorf("Error creating DB connection. Error: %w\n", err)
	}
	defer db.Close()
	// result, err := db.Exec("INSERT INTO dummy_table (id , created , data)  VALUES (?, ?, ?);", rand.Intn(9999), time.Now(), message)

	// if err != nil {
	// 	return nil, fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	// }
	// fmt.Println(result)

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

	pErr := PrintQuery(m)
	if pErr != nil {
		return nil, fmt.Errorf("Error printing query: %w\n", pErr)

	}
	return m, nil
}

func GetDBConnection(dbFolderLocation string) (*sql.DB, error) {
	return sql.Open("sqlite3", path.Join(dbFolderLocation, "db.sqlite"))
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
