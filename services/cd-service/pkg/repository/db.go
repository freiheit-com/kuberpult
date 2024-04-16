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
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	sqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
	"math/rand"
	"path"
	"time"
)

func GetDBConnection(dbFolderLocation string) (*sql.DB, error) {
	return sql.Open("sqlite3", path.Join(dbFolderLocation, "db.sqlite")) //not clear on what is needed for the user and password
}

func RunDBMigrations(ctx context.Context, dbFolderLocation string) {
	db, err := GetDBConnection(dbFolderLocation)
	if err != nil {
		logger.FromContext(ctx).Fatal("DB Error opening DB connection. Error: ", zap.Error(err))
		return
	}
	defer db.Close()

	driver, err := sqlite.WithInstance(db, &sqlite.Config{
		DatabaseName:    "",
		MigrationsTable: "",
		NoTxWrap:        false,
	})
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB driver. Error: ", zap.Error(err))
		return
	}

	migrationsSrc, err := (&file.File{PartialDriver: iofs.PartialDriver{}}).Open(path.Join(dbFolderLocation, "migrations"))
	if err != nil {
		logger.FromContext(ctx).Fatal("Error opening DB migrations. Error: ", zap.Error(err))
		return
	}
	m, err := migrate.NewWithInstance("file", migrationsSrc, "sqlite3", driver)
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating migration instance. Error: ", zap.Error(err))
		return
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			logger.FromContext(ctx).Fatal("Error running DB migrations. Error: ", zap.Error(err))
		}
	}
}

func RetrieveDatabaseInformation(ctx context.Context, databaseLocation string) *sql.Rows {
	db, err := GetDBConnection(databaseLocation)

	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB connection. Error: ", zap.Error(err))
		return nil
	}
	defer db.Close()
	result, err := db.Query("SELECT * FROM dummy_table;")

	if err != nil {
		logger.FromContext(ctx).Warn("Error querying the database. Error: ", zap.Error(err))
		return nil
	}
	return result
}
func InsertDatabaseInformation(ctx context.Context, databaseLocation string, message string) (sql.Result, error) {
	db, err := GetDBConnection(databaseLocation)
	if err != nil {
		logger.FromContext(ctx).Fatal("Error creating DB connection. Error: ", zap.Error(err))
		return nil, err
	}
	defer db.Close()
	result, err := db.Exec("INSERT INTO dummy_table (id , created , data)  VALUES (?, ?, ?);", rand.Intn(9999), time.Now(), message)
	if err != nil {
		logger.FromContext(ctx).Warn("Error inserting information into DB. Error: ", zap.Error(err))
		return nil, err
	}

	if err != nil {
		logger.FromContext(ctx).Warn("Error querying the database. Error: ", zap.Error(err))
		return nil, err
	}
	return result, nil
}

func PrintQuery(ctx context.Context, res *sql.Rows) {
	var (
		id   int
		date []byte
		name string
	)

	for res.Next() {
		err := res.Scan(&id, &date, &name)
		if err != nil {
			logger.FromContext(ctx).Warn("Error retrieving information from query. Error: ", zap.Error(err))
			return
		}
		fmt.Println(id, string(date), name)
	}

}
