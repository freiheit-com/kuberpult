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
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/event"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"path"
	"reflect"
	"slices"
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

func (h *DBHandler) getMigrationHandler() (*migrate.Migrate, error) {
	if h.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, h.DbName, *h.DBDriver)
	} else if h.DriverName == "sqlite3" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, "", *h.DBDriver) //FIX ME
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", h.DriverName)
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

func (h *DBHandler) AdaptQuery(query string) string {
	if h.DriverName == "postgres" {
		return SqliteToPostgresQuery(query)
	} else if h.DriverName == "sqlite3" {
		return query
	}
	panic(fmt.Errorf("AdaptQuery: invalid driver: %s", h.DriverName))
}

// SqliteToPostgresQuery just replaces all "?" into "$1", "$2", etc
func SqliteToPostgresQuery(query string) string {
	var q = query
	var i = 1
	for strings.Contains(q, "?") {
		q = strings.Replace(q, "?", fmt.Sprintf("$%d", i), 1)
		i++
	}
	return q
}

type DBFunction func(ctx context.Context) error

func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// WithTransaction opens a transaction, runs `f` and then calls either Commit or Rollback
func (h *DBHandler) WithTransaction(ctx context.Context, f DBFunction) error {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
		// we ignore the error returned from Rollback() here,
		// because it is always set when Commit() was successful
	}(tx)

	err = f(ctx)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBWriteAllApplications(ctx context.Context, previousVersion int64, applications []string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteAllApplications")
	defer span.Finish()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction. Error: %w", err)
	}
	slices.Sort(applications) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, _ := json.Marshal(AllApplicationsJson{
		Apps: applications,
	})
	insertQuery := h.AdaptQuery("INSERT INTO all_apps (version , created , json)  VALUES (?, ?, ?);")
	logger.FromContext(ctx).Sugar().Warnf("Query: %s", insertQuery)
	//fmt.Printf("Query: %s", insertQuery)
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousVersion+1,
		time.Now(),
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) writeEvent(ctx context.Context, eventType string, sourceCommitHash string, eventJson []byte) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentEvent")
	defer span.Finish()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction. Error: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO events (created , commitHash, eventType, json)  VALUES (?, ?, ?, ?);")
	logger.FromContext(ctx).Sugar().Warnf("Query: %s", insertQuery)
	//fmt.Printf("Query: %s", insertQuery)
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		time.Now(),
		sourceCommitHash,
		eventType,
		eventJson)

	if err != nil {
		return fmt.Errorf("Error inserting information into DB. Error: %w\n", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBWriteDeploymentEvent(ctx context.Context, sourceCommitHash, email string, deployment *event.Deployment) error {

	dataJson, err := json.Marshal(event.Deployment{
		Application:                 deployment.Application,
		Environment:                 deployment.Environment,
		SourceTrainEnvironmentGroup: deployment.SourceTrainEnvironmentGroup,
		SourceTrainUpstream:         deployment.SourceTrainUpstream,
	})

	if err != nil {
		return fmt.Errorf("error converting deployment event data to json. Error: %w\n", err)
	}

	metadataJson, err := json.Marshal(event.Metadata{
		AuthorEmail: email,
	})

	if err != nil {
		return fmt.Errorf("error converting deployment event metadata to json. Error: %w\n", err)
	}
	jsonToInsert, _ := json.Marshal(event.EventJson{
		DataJson:     string(dataJson),
		MetadataJson: string(metadataJson),
	})
	return h.writeEvent(ctx, "deployment", sourceCommitHash, jsonToInsert)
}

// DBSelectAllApplications returns (nil, nil) if there are no rows
func (h *DBHandler) DBSelectAllEventsForCommit(ctx context.Context, commitHash string) ([]EventRow, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEvents")
	defer span.Finish()

	query := h.AdaptQuery("SELECT created, commitHash, eventType, json FROM events WHERE commitHash = (?) ORDER BY created DESC;")
	span.SetTag("query", query)

	logger.FromContext(ctx).Sugar().Warnf("Query: %s", query)
	//fmt.Printf("Query: %s", insertQuery)
	span.SetTag("query", query)

	rows, err := h.DB.QueryContext(ctx, query, commitHash)

	if err != nil {
		return nil, fmt.Errorf("Error querying DB. Error: %w\n", err)
	}

	var result []EventRow

	for rows.Next() {
		var row = EventRow{}
		err := rows.Scan(&row.Created, &row.CommitHash, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning row from DB. Error: %w\n", err)
		}

		result = append(result, row)
	}
	return result, nil
}

// DBSelectAllApplications returns (nil, nil) if there are no rows
func (h *DBHandler) DBSelectAllApplications(ctx context.Context) (*AllApplicationsGo, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer span.Finish()
	query := "SELECT version, created, json FROM all_apps ORDER BY version DESC LIMIT 1;"
	span.SetTag("query", query)
	rows := h.DB.QueryRowContext(ctx, query)
	result := AllApplicationsRow{
		version: 0,
		created: time.Time{},
		data:    "",
	}
	err := rows.Scan(&result.version, &result.created, &result.data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning row from DB. Error: %w\n", err)
	}

	//exhaustruct:ignore
	var resultJson = AllApplicationsJson{}
	err = json.Unmarshal(([]byte)(result.data), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, result.data)
	}
	var resultGo = AllApplicationsGo{
		Version:             result.version,
		Created:             result.created,
		AllApplicationsJson: AllApplicationsJson{Apps: resultJson.Apps},
	}
	return &resultGo, nil
}

func (h *DBHandler) RunCustomMigrations(ctx context.Context, repo Repository) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	return h.RunCustomMigrationAllTables(ctx, repo)
}

func (h *DBHandler) RunCustomMigrationAllTables(ctx context.Context, repo Repository) error {
	return h.WithTransaction(ctx, func(ctx context.Context) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAllApplications(ctx)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}

		allAppsRepo, err := repo.State().GetApplicationsFromFile()
		if err != nil {
			return fmt.Errorf("could not get applications to run custom migrations: %v", err)
		}
		var version int64
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
		// if there is any difference, we assume the manifest wins over the database state,
		// so we use `allAppsRepo`:
		return h.DBWriteAllApplications(ctx, version, allAppsRepo)
	})
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

type EventRow struct {
	Created    time.Time
	CommitHash string
	EventType  string
	EventJson  string
}
