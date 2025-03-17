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

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/valid"

	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	config "github.com/freiheit-com/kuberpult/pkg/config"
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
	WriteEslOnly   bool
	SSLMode        string

	MaxIdleConnections uint
	MaxOpenConnections uint
}

type DBHandler struct {
	DbName         string
	DriverName     string
	MigrationsPath string
	DB             *sql.DB
	DBDriver       *database.Driver

	/*
		There are 3 modes:
		1) DBHandler==nil: do not write anything to the DB
		2) DBHandler!=nil && WriteEslOnly==true: write only the ESL table to the database. Stores all incoming data in the DB, but does not read the DB.
		3) DBHandler!=nil && WriteEslOnly==false: write everything to the database.
	*/
	WriteEslOnly bool
}

type EslVersion int64
type TransformerID EslVersion

const (
	MigrationCommitEventUUID = "00000000-0000-0000-0000-000000000000"
	MigrationCommitEventHash = "0000000000000000000000000000000000000000"
	WhereInBatchMax          = 1024
)

func (h *DBHandler) ShouldUseEslTable() bool {
	return h != nil
}

// ShouldUseOtherTables returns true if the db is enabled and WriteEslOnly=false
// ShouldUseOtherTables should never be used in the manifest-repo-export-service.
func (h *DBHandler) ShouldUseOtherTables() bool {
	return h != nil && !h.WriteEslOnly
}

// AllowParallelTransactions returns true if the underlying database does allow multiple transactions in parallel
// Postgres allows this, and sqlite does not (in sqlite you would need to open a new connection).
func (h *DBHandler) AllowParallelTransactions() bool {
	return h.DriverName == "postgres"
}

func Connect(ctx context.Context, cfg DBConfig) (*DBHandler, error) {
	db, driver, err := GetConnectionAndDriverWithRetries(ctx, cfg)

	if err != nil {
		return nil, err
	}
	var handler = &DBHandler{
		DbName:         cfg.DbName,
		DriverName:     cfg.DriverName,
		MigrationsPath: cfg.MigrationsPath,
		DB:             db,
		DBDriver:       &driver,
		WriteEslOnly:   cfg.WriteEslOnly,
	}
	return handler, nil
}

func GetDBConnection(cfg DBConfig) (*sql.DB, error) {
	if cfg.DriverName == "postgres" {
		dbURI := fmt.Sprintf("host=%s user=%s password=%s port=%s database=%s sslmode=%s",
			cfg.DbHost, cfg.DbUser, cfg.DbPassword, cfg.DbPort, cfg.DbName, cfg.SSLMode)

		dbPool, err := sql.Open(cfg.DriverName, dbURI)
		if err != nil {
			return nil, fmt.Errorf("sql.Open: %w", err)
		}
		dbPool.SetConnMaxLifetime(5 * time.Minute)
		dbPool.SetMaxOpenConns(int(cfg.MaxOpenConnections))
		dbPool.SetMaxIdleConns(int(cfg.MaxIdleConnections))
		return dbPool, nil
	} else if cfg.DriverName == "sqlite3" {
		return sql.Open("sqlite3", path.Join(cfg.DbHost, "db.sqlite?_foreign_keys=on"))
	}
	return nil, fmt.Errorf("driver: only postgres and sqlite3 are supported, but not '%s'", cfg.DriverName)
}

func GetConnectionAndDriverWithRetries(ctx context.Context, cfg DBConfig) (*sql.DB, database.Driver, error) {
	var l = logger.FromContext(ctx).Sugar()
	var db *sql.DB
	var err error
	var driver database.Driver
	for i := 10; i > 0; i-- {
		db, driver, err = GetConnectionAndDriver(cfg)
		if err == nil {
			return db, driver, nil
		}
		if i > 0 {
			d := time.Second * 10
			l.Warnf("could not connect to db, will try again in %v for %d more times, error: %v", d, i, err)
			time.Sleep(d)
		}
	}
	return nil, nil, err

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

func RunDBMigrations(ctx context.Context, cfg DBConfig) error {
	d, err := Connect(ctx, cfg)
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

func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func closeRows(rows *sql.Rows) error {
	err := rows.Close()
	if err != nil {
		return fmt.Errorf("row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return fmt.Errorf("row has error: %v\n", err)
	}
	return nil
}

type EventType string

const (
	EvtCreateApplicationVersion         EventType = "CreateApplicationVersion"
	EvtDeployApplicationVersion         EventType = "DeployApplicationVersion"
	EvtCreateUndeployApplicationVersion EventType = "CreateUndeployApplicationVersion"
	EvtUndeployApplication              EventType = "UndeployApplication"
	EvtDeleteEnvFromApp                 EventType = "DeleteEnvFromApp"
	EvtCreateEnvironmentLock            EventType = "CreateEnvironmentLock"
	EvtDeleteEnvironmentLock            EventType = "DeleteEnvironmentLock"
	EvtCreateEnvironmentTeamLock        EventType = "CreateEnvironmentTeamLock"
	EvtDeleteEnvironmentTeamLock        EventType = "DeleteEnvironmentTeamLock"
	EvtCreateEnvironmentGroupLock       EventType = "CreateEnvironmentGroupLock"
	EvtDeleteEnvironmentGroupLock       EventType = "DeleteEnvironmentGroupLock"
	EvtCreateEnvironment                EventType = "CreateEnvironment"
	EvtDeleteEnvironment                EventType = "DeleteEnvironment"
	EvtCreateEnvironmentApplicationLock EventType = "CreateEnvironmentApplicationLock"
	EvtDeleteEnvironmentApplicationLock EventType = "DeleteEnvironmentApplicationLock"
	EvtReleaseTrain                     EventType = "ReleaseTrain"
	EvtMigrationTransformer             EventType = "MigrationTransformer"
	EvtEnvReleaseTrain                  EventType = "EnvReleaseTrain"
	EvtCleanupOldApplicationVersions    EventType = "CleanupOldApplicationVersions"
	EvtSkippedServices                  EventType = "SkippedServices"
)

// ESL EVENTS

type ESLMetadata struct {
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
}

// DBWriteEslEventInternal writes one event to the event-sourcing-light table, taking arbitrary data as input
func (h *DBHandler) DBWriteEslEventInternal(ctx context.Context, eventType EventType, tx *sql.Tx, data interface{}, metadata ESLMetadata) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	dataMap, err := convertObjectToMap(data)

	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	metadataMap, err := convertObjectToMap(metadata)
	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	dataMap["metadata"] = metadataMap
	jsonToInsert, err := json.Marshal(dataMap)
	if err != nil {
		return fmt.Errorf("could not marshal combined json data: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (created, event_type , json)  VALUES (?, ?, ?);")

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEslEventInternal unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		eventType,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteEslEventWithJson(ctx context.Context, tx *sql.Tx, eventType EventType, data string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (created, event_type , json)  VALUES (?, ?, ?);")

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEslEventInternal unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		eventType,
		data)

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w\n", err)
	}
	return nil
}

func convertObjectToMap(obj interface{}) (map[string]interface{}, error) {
	if obj == nil {
		return map[string]interface{}{}, nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var result = make(map[string]interface{})
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type EslEventRow struct {
	EslVersion EslVersion
	Created    time.Time
	EventType  EventType
	EventJson  string
}

type EslFailedEventRow struct {
	EslVersion            EslVersion
	Created               time.Time
	EventType             EventType
	EventJson             string
	Reason                string
	TransformerEslVersion EslVersion
}

// DBDiscoverCurrentEsldID: Returns the current sequence number of event_sourcing_light table.
// Next value should be the returned on + 1
func (h *DBHandler) DBDiscoverCurrentEsldID(ctx context.Context, tx *sql.Tx) (*int, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBDiscoverCurrentEsldID: no transaction provided")
	}
	var selectQuery string
	if h.DriverName == "postgres" {
		selectQuery = h.AdaptQuery("SELECT last_value from event_sourcing_light_eslversion_seq;")

	} else if h.DriverName == "sqlite3" {
		selectQuery = h.AdaptQuery("SELECT seq FROM SQLITE_SEQUENCE WHERE name='event_sourcing_light';")
	} else {
		return nil, fmt.Errorf("Driver: '%s' not supported.\n", h.DriverName)
	}
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get current eslVersion. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var value *int
	value = new(int)
	if rows.Next() {
		err := rows.Scan(value)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error table for next eslVersion. Error: %w\n", err)
		}
	} else {
		value = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// DBReadEslEventInternal returns either the first or the last row of the esl table
func (h *DBHandler) DBReadEslEventInternal(ctx context.Context, tx *sql.Tx, firstRow bool) (*EslEventRow, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadEslEventInternal: no transaction provided")
	}
	sort := "DESC"
	if firstRow {
		sort = "ASC"
	}
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslVersion, created, event_type , json FROM event_sourcing_light ORDER BY eslVersion %s LIMIT 1;", sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var row = &EslEventRow{
		EslVersion: 0,
		Created:    time.Unix(0, 0),
		EventType:  "",
		EventJson:  "",
	}
	if rows.Next() {
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning event_sourcing_light row from DB. Error: %w\n", err)
		}
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DBReadEslEventLaterThan returns the first row of the esl table that has an eslVersion > the given eslVersion
func (h *DBHandler) DBReadEslEventLaterThan(ctx context.Context, tx *sql.Tx, eslVersion EslVersion) (*EslEventRow, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadEslEventLaterThan")
	defer span.Finish()

	sort := "ASC"
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslVersion, created, event_type, json FROM event_sourcing_light WHERE eslVersion > (?) ORDER BY eslVersion %s LIMIT 1;", sort))
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		eslVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error for event_sourcing_light: %v", err)
		}
	}(rows)
	var row = &EslEventRow{
		EslVersion: 0,
		Created:    time.Unix(0, 0),
		EventType:  "",
		EventJson:  "",
	}
	if !rows.Next() {
		row = nil
	} else {
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("event_sourcing_light: Error scanning row from DB. Error: %w\n", err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) WriteEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, eventuuid string, eventType event.EventType, sourceCommitHash string, eventJson []byte) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "WriteEvent")
	defer span.Finish()

	insertQuery := h.AdaptQuery("INSERT INTO commit_events (uuid, timestamp, commitHash, eventType, json, transformereslVersion)  VALUES (?, ?, ?, ?, ?, ?);")

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("WriteEvent unable to get transaction timestamp: %w", err)
	}

	span.SetTag("query", insertQuery)

	rawUUID, err := timeuuid.ParseUUID(eventuuid)
	if err != nil {
		return fmt.Errorf("error parsing UUID. Error: %w", err)
	}
	_, err = transaction.Exec(
		insertQuery,
		rawUUID.String(),
		*now,
		sourceCommitHash,
		eventType,
		eventJson,
		transformerID)

	if err != nil {
		return fmt.Errorf("Error inserting event information into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteNewReleaseEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, releaseVersion uint64, uuid, sourceCommitHash string, newReleaseEvent *event.NewRelease) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentEvent")
	defer span.Finish()

	metadata := event.Metadata{
		Uuid:           uuid,
		EventType:      string(event.EventTypeNewRelease),
		ReleaseVersion: releaseVersion,
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     newReleaseEvent,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling lock new release event to Json. Error: %v\n", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeNewRelease, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteLockPreventedDeploymentEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, uuid, sourceCommitHash string, lockPreventedDeploymentEvent *event.LockPreventedDeployment) error {
	metadata := event.Metadata{
		Uuid:           uuid,
		EventType:      string(event.EventTypeLockPreventedDeployment),
		ReleaseVersion: 0, // don't care about release version for this event
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     lockPreventedDeploymentEvent,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling lock prevented deployment event to Json. Error: %v\n", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeLockPreventedDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteReplacedByEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, uuid, sourceCommitHash string, replacedBy *event.ReplacedBy) error {
	metadata := event.Metadata{
		Uuid:           uuid,
		EventType:      string(event.EventTypeReplaceBy),
		ReleaseVersion: 0, // don't care about release version for this event
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     replacedBy,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling replacedBys event to Json. Error: %v\n", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeReplaceBy, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteDeploymentEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, uuid, sourceCommitHash string, deployment *event.Deployment) error {
	metadata := event.Metadata{
		Uuid:           uuid,
		EventType:      string(event.EventTypeDeployment),
		ReleaseVersion: 0, // don't care about release version for this event
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     deployment,
		EventMetadata: metadata,
	})
	if !valid.SHA1CommitID(sourceCommitHash) {
		return fmt.Errorf("refusing to write deployment event without commit hash for transformer %v with uuid %s",
			transformerID,
			uuid,
		)
	}

	if err != nil {
		return fmt.Errorf("error marshalling deployment event to Json. Error: %v\n", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBSelectAnyEvent(ctx context.Context, transaction *sql.Tx) (*EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAnyEvent: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyEvent")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion FROM commit_events ORDER BY timestamp DESC LIMIT 1;")
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query)
	return h.processSingleEventsRow(ctx, rows, err)
}

func (h *DBHandler) DBContainsMigrationCommitEvent(ctx context.Context, transaction *sql.Tx) (bool, error) {
	if h == nil {
		return false, nil
	}
	if transaction == nil {
		return false, fmt.Errorf("DBContainsMigrationCommitEvent: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBContainsMigrationCommitEvent")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion FROM commit_events WHERE commitHash = (?) ORDER BY timestamp DESC LIMIT 1;")
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query, MigrationCommitEventHash)

	row, err := h.processSingleEventsRow(ctx, rows, err)

	if err != nil {
		return false, err
	}

	return row != nil, nil
}

func (h *DBHandler) DBSelectAllCommitEventsForTransformer(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, eventType event.EventType, limit uint) ([]event.DBEventGo, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllCommitEventsForTransformer: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllCommitEventsForTransformer")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion FROM commit_events WHERE eventType = (?) AND transformereslVersion = (?) ORDER BY timestamp DESC LIMIT ?;")
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, string(eventType), transformerID, limit)
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	var result []event.DBEventGo
	for rows.Next() {
		//exhaustruct:ignore
		var row = &EventRow{}
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
		}

		eventGo, err := event.UnMarshallEvent(row.EventType, row.EventJson)
		if err != nil {
			return nil, fmt.Errorf("Could not unmarshall commit event: %v\n", err)
		}
		result = append(result, eventGo)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return result, nil
}

func (h *DBHandler) processSingleEventsRow(ctx context.Context, rows *sql.Rows, err error) (*EventRow, error) {
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = &EventRow{}
	if rows.Next() {
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
		}
	} else {
		row = nil
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return row, nil
}

func (h *DBHandler) DBSelectAllEventsForCommit(ctx context.Context, transaction *sql.Tx, commitHash string, pageNumber, pageSize uint64) ([]EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEventsForCommit")
	defer span.Finish()

	// NOTE: We add one so we know if there is more to load
	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion FROM commit_events WHERE commitHash = (?) ORDER BY timestamp ASC LIMIT (?) OFFSET (?);")
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, commitHash, pageSize+1, pageNumber*pageSize)
	return processAllCommitEventRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectAllCommitEventsForTransformerID(ctx context.Context, transaction *sql.Tx, transformerID TransformerID) ([]EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEventsForCommit")
	defer span.Finish()

	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		WHERE transformereslVersion = (?)
		ORDER BY timestamp DESC, uuid ASC
		LIMIT 100;`)
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, transformerID)
	return processAllCommitEventRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectAllLockPreventedEventsForTransformerID(ctx context.Context, transaction *sql.Tx, transformerID TransformerID) ([]EventRow, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEventsForCommit")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion FROM commit_events WHERE transformereslVersion = (?) AND eventtype = (?) ORDER BY timestamp DESC LIMIT 100;")
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, transformerID, string(event.EventTypeLockPreventedDeployment))
	return processAllCommitEventRow(ctx, rows, err)
}

func processAllCommitEventRow(ctx context.Context, rows *sql.Rows, err error) ([]EventRow, error) {
	if err != nil {
		return nil, fmt.Errorf("Error querying commit_events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("commit_events row could not be closed: %v", err)
		}
	}(rows)

	var result []EventRow

	for rows.Next() {
		row, err := processSingleCommitEventRow(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, *row)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v\n", err)
	}
	return result, nil
}

func processSingleCommitEventRow(rows *sql.Rows) (*EventRow, error) {
	var row = EventRow{
		Uuid:          "",
		Timestamp:     time.Unix(0, 0), //will be overwritten, prevents CI linter from complaining from missing fields
		CommitHash:    "",
		EventType:     "",
		EventJson:     "",
		TransformerID: 0,
	}
	err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning commit_events row from DB. Error: %w\n", err)
	}
	return &row, nil
}

type EnvironmentLock struct {
	EslVersion EslVersion
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   LockMetadata
}

// DBEnvironmentLock Just used to fetch info from DB
type DBEnvironmentLock struct {
	EslVersion EslVersion
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   string
}

type LockMetadata struct {
	CreatedByName  string
	CreatedByEmail string
	Message        string
	CiLink         string
	CreatedAt      time.Time
}

type ReleaseWithManifest struct {
	Version uint64
	/**
	"UndeployVersion=true" means that this version is empty, and has no manifest that could be deployed.
	It is intended to help cleanup old services within the normal release cycle (e.g. dev->staging->production).
	*/
	UndeployVersion bool
	SourceAuthor    string
	SourceCommitId  string
	SourceMessage   string
	CreatedAt       time.Time
	DisplayVersion  string

	Manifests map[string]string // keys: environment; value: manifest
}

type AllDeployments []Deployment
type AllEnvLocks map[string][]EnvironmentLock
type AllReleases map[uint64]ReleaseWithManifest // keys: releaseVersion; value: release with manifests

// WriteAllDeploymentsFun and other functions here are used during migration.
// They are supposed to read data from files in the manifest repo and write it to the databse,
// and therefore need to access the Database.
type WriteAllDeploymentsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllAppLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error

type AllAppLocks map[string]map[string][]ApplicationLock // EnvName-> AppName -> []Locks
type AllTeamLocks map[string]map[string][]TeamLock       // EnvName-> Team -> []Locks
type AllQueuedVersions map[string]map[string]*int64      // EnvName-> AppName -> queuedVersion
type AllCommitEvents map[string][]event.DBEventGo        // CommitId -> uuid -> Event

type WriteAllEnvLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllTeamLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllReleasesFun = func(ctx context.Context, transaction *sql.Tx, app string, dbHandler *DBHandler) error
type WriteAllQueuedVersionsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllEventsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error

// GetAllAppsFun returns a map where the Key is an app name, and the value is a team name of that app
type GetAllAppsFun = func() (map[string]string, error)

// return value is a map from environment name to environment config
type GetAllEnvironmentsFun = func(ctx context.Context) (map[string]config.EnvironmentConfig, error)

func (h *DBHandler) RunCustomMigrations(
	ctx context.Context,
	getAllAppsFun GetAllAppsFun,
	writeAllDeploymentsFun WriteAllDeploymentsFun,
	writeAllReleasesFun WriteAllReleasesFun,
	writeAllEnvLocksFun WriteAllEnvLocksFun,
	writeAllAppLocksFun WriteAllAppLocksFun,
	writeAllTeamLocksFun WriteAllTeamLocksFun,
	getAllEnvironmentsFun GetAllEnvironmentsFun,
	writeAllQueuedVersionsFun WriteAllQueuedVersionsFun,
	writeAllEventsFun WriteAllEventsFun,
) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	err := h.RunCustomMigrationsEventSourcingLight(ctx)
	if err != nil {
		return err
	}
	err = h.RunAllCustomMigrationsForApps(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationDeployments(ctx, writeAllDeploymentsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationReleases(ctx, getAllAppsFun, writeAllReleasesFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvLocks(ctx, writeAllEnvLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationAppLocks(ctx, writeAllAppLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationTeamLocks(ctx, writeAllTeamLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationsCommitEvents(ctx, writeAllEventsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvironments(ctx, getAllEnvironmentsFun)
	if err != nil {
		return err // better wrap the error in a descriptive message?
	}
	return nil
}

func NewNullInt(s *int64) sql.NullInt64 {
	if s == nil {
		return sql.NullInt64{
			Int64: 0,
			Valid: false,
		}
	}
	return sql.NullInt64{
		Int64: *s,
		Valid: true,
	}
}

// CUSTOM MIGRATIONS

func (h *DBHandler) RunCustomMigrationReleases(ctx context.Context, getAllAppsFun GetAllAppsFun, writeAllReleasesFun WriteAllReleasesFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationReleases")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		needsMigrating, err := h.needsReleasesMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}
		allAppsMap, err := getAllAppsFun()
		if err != nil {
			return err
		}
		for app := range allAppsMap {
			l.Infof("processing app %s ...", app)

			err := writeAllReleasesFun(ctx, transaction, app, h)
			if err != nil {
				return fmt.Errorf("could not migrate releases to database: %v", err)
			}
			l.Infof("done with app %s", app)
		}
		return nil
	})
}

func (h *DBHandler) needsReleasesMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allReleasesDb, err := h.DBSelectAnyRelease(ctx, transaction, true)
	if err != nil {
		return true, err
	}
	if allReleasesDb != nil {
		l.Warnf("There are already deployments in the DB - skipping migrations")
		return false, nil
	}
	return true, nil

}

func (h *DBHandler) RunCustomMigrationDeployments(ctx context.Context, getAllDeploymentsFun WriteAllDeploymentsFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationDeployments")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsDeploymentsMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}
		err = getAllDeploymentsFun(ctx, transaction, h)
		if err != nil {
			return fmt.Errorf("could not get current deployments to run custom migrations: %v", err)
		}

		return nil
	})
}

func (h *DBHandler) needsDeploymentsMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allAppsDb, err := h.DBSelectAnyDeployment(ctx, transaction)
	if err != nil {
		return true, err
	}
	if allAppsDb != nil {
		l.Warnf("There are already deployments in the DB - skipping migrations")
		return false, nil
	}
	return true, nil
}

type EventRow struct {
	Uuid          string
	Timestamp     time.Time
	CommitHash    string
	EventType     event.EventType
	EventJson     string
	TransformerID TransformerID
}

func (h *DBHandler) RunCustomMigrationEnvLocks(ctx context.Context, writeAllEnvLocksFun WriteAllEnvLocksFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvLocks")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsEnvLocksMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}
		err = writeAllEnvLocksFun(ctx, transaction, h)
		if err != nil {
			return fmt.Errorf("could not get current environment locks to run custom migrations: %w", err)
		}
		return nil
	})
}

func (h *DBHandler) needsEnvLocksMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allEnvLocksDb, err := h.DBSelectAnyActiveEnvLocks(ctx, transaction)
	if err != nil {
		return true, err
	}
	if allEnvLocksDb != nil {
		l.Infof("There are already environment locks in the DB - skipping migrations")
		return false, nil
	}
	return true, nil
}

func (h *DBHandler) RunCustomMigrationAppLocks(ctx context.Context, writeAllAppLocksFun WriteAllAppLocksFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationAppLocks")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsAppLocksMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}
		err = writeAllAppLocksFun(ctx, transaction, h)
		if err != nil {
			return fmt.Errorf("could not get current application locks to run custom migrations: %w", err)
		}
		return nil
	})
}

func (h *DBHandler) needsAppLocksMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allAppLocksDb, err := h.DBSelectAnyActiveAppLock(ctx, transaction)
	if err != nil {
		return true, err
	}
	if allAppLocksDb != nil {
		l.Infof("There are already application locks in the DB - skipping migrations")
		return false, nil
	}
	return true, nil
}

func (h *DBHandler) RunCustomMigrationTeamLocks(ctx context.Context, writeAllTeamLocksFun WriteAllTeamLocksFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationTeamLocks")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsTeamLocksMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}

		err = writeAllTeamLocksFun(ctx, transaction, h)
		if err != nil {
			return fmt.Errorf("could not get current team locks to run custom migrations: %w", err)
		}
		return nil
	})
}

func (h *DBHandler) needsTeamLocksMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allTeamLocksDb, err := h.DBSelectAnyActiveTeamLock(ctx, transaction)
	if err != nil {
		return true, err
	}
	if allTeamLocksDb != nil {
		l.Infof("There are already team locks in the DB - skipping migrations")
		return false, nil
	}
	return true, nil
}

func (h *DBHandler) RunCustomMigrationsCommitEvents(ctx context.Context, writeAllEvents WriteAllEventsFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsCommitEventsMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}

		err = writeAllEvents(ctx, transaction, h)
		if err != nil {
			return fmt.Errorf("could not get current commit events to run custom migrations: %w", err)
		}
		//Migration event
		err = h.WriteEvent(ctx, transaction, 0, MigrationCommitEventUUID, event.EventTypeDBMigrationEventType, MigrationCommitEventHash, []byte("{}"))
		if err != nil {
			return fmt.Errorf("error writing migration commit event to the database: %w\n", err)
		}
		return nil
	})
}

func (h *DBHandler) needsCommitEventsMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()

	//Checks for 'migration' commit event with hash 0000(...)0000
	contains, err := h.DBContainsMigrationCommitEvent(ctx, transaction)
	if err != nil {
		return true, err
	}
	if contains {
		l.Infof("detected migration commit event on the database - skipping migrations")
		return false, nil
	}
	return true, nil
}

// For commit_events migrations, we need some transformer to be on the database before we run their migrations.
func (h *DBHandler) RunCustomMigrationsEventSourcingLight(ctx context.Context) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationsEventSourcingLight")
		defer span.Finish()

		needsMigrating, err := h.NeedsEventSourcingLightMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}

		return h.DBWriteMigrationsTransformer(ctx, transaction)
	})
}

type CheckFun = func(handler *DBHandler, ctx context.Context, transaction *sql.Tx) (bool, error)

func (h *DBHandler) NeedsEventSourcingLightMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	eslEvent, err := h.DBReadEslEventInternal(ctx, transaction, true) //true sorts by asc
	if err != nil {
		return true, err
	}
	if eslEvent != nil && eslEvent.EslVersion == 0 { //Check if there is a 0th transformer already
		l.Infof("Found Migrations transformer on database.")
		return false, nil
	}
	return true, nil
}

func (h *DBHandler) DBWriteMigrationsTransformer(ctx context.Context, transaction *sql.Tx) error {
	if h == nil {
		return nil
	}
	if transaction == nil {
		return fmt.Errorf("DBWriteMigrationsTransformer: no transaction provided")
	}

	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteMigrationsTransformer")
	defer span.Finish()

	dataMap := make(map[string]interface{})
	metadata := ESLMetadata{AuthorName: "Migration", AuthorEmail: "Migration"}
	metadataMap, err := convertObjectToMap(metadata)
	if err != nil {
		return fmt.Errorf("could not convert object to map: %w", err)
	}
	dataMap["metadata"] = metadataMap
	dataMap["eslVersion"] = 0
	jsonToInsert, err := json.Marshal(dataMap)

	if err != nil {
		return fmt.Errorf("could not marshal json transformer: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (eslversion, created, event_type, json) VALUES (0, ?, ?, ?);")
	ts, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("DBWriteMigrationsTransformer unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err2 := transaction.Exec(
		insertQuery,
		ts,
		EvtMigrationTransformer,
		jsonToInsert)
	if err2 != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w", err2)
	}
	return nil
}

func (h *DBHandler) RunAllCustomMigrationsForApps(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunAllCustomMigrationsForApps")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needMigrating, err := h.needsAppsMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needMigrating {
			return nil
		}

		allAppsRepo, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get applications from manifest to run custom migrations: %v", err)
		}

		err = h.runCustomMigrationApps(ctx, transaction, &allAppsRepo)
		if err != nil {
			return fmt.Errorf("could not perform apps table migration: %v\n", err)
		}
		return nil
	})
}

func (h *DBHandler) needsAppsMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	l := logger.FromContext(ctx).Sugar()
	allAppsDb, err := h.DBSelectAllApplications(ctx, transaction)
	if err != nil {
		l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
		return false, err
	}
	return len(allAppsDb) == 0, nil
}

// runCustomMigrationApps : Runs custom migrations for provided apps.
func (h *DBHandler) runCustomMigrationApps(ctx context.Context, transaction *sql.Tx, appsMap *map[string]string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "runCustomMigrationApps")
	defer span.Finish()

	for app, team := range *appsMap {
		err := h.DBInsertOrUpdateApplication(ctx, transaction, app, AppStateChangeMigrate, DBAppMetaData{Team: team})
		if err != nil {
			return fmt.Errorf("could not write dbApp %s: %v", app, err)
		}
	}
	return nil
}

// ENV LOCKS

func (h *DBHandler) DBSelectAnyActiveEnvLocks(ctx context.Context, tx *sql.Tx) (*AllEnvLocksGo, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyActiveEnvLocks")
	defer span.Finish()

	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, json FROM all_env_locks ORDER BY version DESC LIMIT 1;")
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query environment_locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = AllEnvLocksRow{}
	if rows.Next() {
		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment lock row from DB. Error: %w\n", err)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		//exhaustruct:ignore
		var dataJson = AllEnvLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &dataJson)
		if err != nil {
			return nil, fmt.Errorf("Error scanning application lock row from DB. Error: %w\n", err)
		}
		return &AllEnvLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AllEnvLocksJson: AllEnvLocksJson{EnvLocks: dataJson.EnvLocks}}, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error
}

func (h *DBHandler) DBSelectEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) (*EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadEslEventLaterThan")
	defer span.Finish()

	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
			" FROM environment_locks " +
			" WHERE envName=? AND lockID=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		lockID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query environment locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &EnvironmentLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		}, nil
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error

}

func (h *DBHandler) DBWriteEnvironmentLock(ctx context.Context, tx *sql.Tx, lockID, environment string, metadata LockMetadata) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLock")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLock: no transaction provided")
	}

	var previousVersion EslVersion

	existingEnvLock, err := h.DBSelectEnvironmentLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing environment lock: %w\n", err)
	}

	if existingEnvLock == nil {
		previousVersion = 0
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	envLock := EnvironmentLock{
		EslVersion: 0,
		LockID:     lockID,
		Created:    time.Time{},
		Env:        environment,
		Metadata:   metadata,
		Deleted:    false,
	}
	return h.DBWriteEnvironmentLockInternal(ctx, tx, envLock, previousVersion)
}

func (h *DBHandler) DBWriteEnvironmentLockInternal(ctx context.Context, tx *sql.Tx, envLock EnvironmentLock, previousEslVersion EslVersion) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLockInternal")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLockInternal: no transaction provided")
	}

	now, err := h.DBReadTransactionTimestamp(ctx, tx)

	if err != nil {
		return fmt.Errorf("DBWriteEnvironmentLockInternal unable to get transaction timestamp: %w", err)
	}

	envLock.Created = *now

	jsonToInsert, err := json.Marshal(envLock.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO environment_locks (eslVersion, created, lockID, envName, deleted, metadata) VALUES (?, ?, ?, ?, ?, ?);")

	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		*now,
		envLock.LockID,
		envLock.Env,
		envLock.Deleted,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write environment lock into DB. Error: %w\n", err)
	}
	return nil
}

// DBSelectEnvLockHistory returns the last N events associated with some lock on some environment. Currently only used in testing.
func (h *DBHandler) DBSelectEnvLockHistory(ctx context.Context, tx *sql.Tx, environmentName, lockID string, limit int) ([]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvLocks")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvLocks: no transaction provided")
	}

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
				" FROM environment_locks " +
				" WHERE envName=? AND lockID=?" +
				" ORDER BY eslVersion DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w\n", err)
	}
	envLocks := make([]EnvironmentLock, 0)
	for rows.Next() {
		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    true,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		envLocks = append(envLocks, EnvironmentLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

func (h *DBHandler) DBSelectAllEnvLocksOfAllEnvs(ctx context.Context, tx *sql.Tx) (map[string][]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvLocksOfAllEnvs")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvLocksOfAllEnvs: no transaction provided")
	}

	selectQuery := h.AdaptQuery(
		`
		SELECT
			environment_locks.eslversion,
			environment_locks.created,
			environment_locks.lockID,
			environment_locks.envName,
			environment_locks.metadata,
			environment_locks.deleted
		FROM (
			SELECT
			MAX(eslVersion) AS latest,
			envName,
			lockid
		FROM
			"environment_locks"
		GROUP BY
			envName, lockid
		) AS latest
		JOIN
			environment_locks AS environment_locks
		ON
		latest.latest=environment_locks.eslVersion
		AND latest.envName=environment_locks.envName
		AND latest.lockid=environment_locks.lockid
		WHERE deleted = false;`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w\n", err)
	}
	envLocks := make(map[string][]EnvironmentLock)
	for rows.Next() {
		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    false,
			Metadata:   "",
		}

		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		if _, ok := envLocks[row.Env]; !ok {
			envLocks[row.Env] = make([]EnvironmentLock, 0)
		}
		envLocks[row.Env] = append(envLocks[row.Env], EnvironmentLock{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			LockID:     row.LockID,
			Env:        row.Env,
			Deleted:    row.Deleted,
			Metadata:   resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return envLocks, nil
}

func (h *DBHandler) DBSelectAllEnvironmentLocks(ctx context.Context, tx *sql.Tx, environment string) (*AllEnvLocksGo, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvironmentLocks")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvironmentLocks: no transaction provided")
	}
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, json FROM all_env_locks WHERE environment = ? ORDER BY version DESC LIMIT 1;")
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(ctx, selectQuery, environment)
	if err != nil {
		return nil, fmt.Errorf("could not query all env locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	if rows.Next() {
		var row = AllEnvLocksRow{
			Version:     0,
			Created:     time.Time{},
			Environment: "",
			Data:        "",
		}

		err := rows.Scan(&row.Version, &row.Created, &row.Environment, &row.Data)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = AllEnvLocksJson{}
		err = json.Unmarshal(([]byte)(row.Data), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Data)
		}

		var resultGo = AllEnvLocksGo{
			Version:         row.Version,
			Created:         row.Created,
			Environment:     row.Environment,
			AllEnvLocksJson: AllEnvLocksJson{EnvLocks: resultJson.EnvLocks},
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &resultGo, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *DBHandler) DBSelectEnvironmentLockSet(ctx context.Context, tx *sql.Tx, environment string, lockIDs []string) ([]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentLockSet")
	defer span.Finish()

	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvironmentLockSet: no transaction provided")
	}

	var envLocks []EnvironmentLock
	var rows *sql.Rows
	defer func(rows *sql.Rows) {
		if rows == nil {
			return
		}
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	//Get the latest change to each lock
	for _, id := range lockIDs {
		var err error
		selectQuery := h.AdaptQuery(
			"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
				" FROM environment_locks " +
				" WHERE envName=? AND lockID=? " +
				" ORDER BY eslVersion DESC " +
				" LIMIT 1;")
		rows, err = tx.QueryContext(ctx, selectQuery, environment, id)
		if err != nil {
			return nil, fmt.Errorf("could not query environment locks table from DB. Error: %w\n", err)
		}

		var row = DBEnvironmentLock{
			EslVersion: 0,
			Created:    time.Time{},
			LockID:     "",
			Env:        "",
			Deleted:    false,
			Metadata:   "",
		}
		if rows.Next() {
			err = rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
			}

			//exhaustruct:ignore
			var resultJson = LockMetadata{}
			err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
			if err != nil {
				return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
			}
			envLocks = append(envLocks, EnvironmentLock{
				EslVersion: row.EslVersion,
				Created:    row.Created,
				LockID:     row.LockID,
				Env:        row.Env,
				Deleted:    row.Deleted,
				Metadata:   resultJson,
			})
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	err := closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

func (h *DBHandler) DBWriteAllEnvironmentLocks(ctx context.Context, transaction *sql.Tx, previousVersion int64, environment string, lockIds []string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteAllEnvironmentLocks")
	defer span.Finish()
	slices.Sort(lockIds) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllEnvLocksJson{
		EnvLocks: lockIds,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_env_locks (version , created, environment, json)  VALUES (?, ?, ?, ?);")

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("DBWriteAllEnvironmentLocks unable to get transaction timestamp: %w", err)
	}

	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		*now,
		environment,
		jsonToInsert)
	if err != nil {
		return fmt.Errorf("could not insert all env locks into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironmentLock")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteEnvironmentLock: no transaction provided")
	}
	var previousVersion EslVersion

	//See if there is an existing lock with the same lock id in this environment. If it exists, just add a +1 to the eslversion
	existingEnvLock, err := h.DBSelectEnvironmentLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing environment lock: %w\n", err)
	}

	if existingEnvLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' does not exist. Continuing anyway.", lockID, environment)
		return nil
	}

	if existingEnvLock.Deleted {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' has already been deleted. Continuing anyway.", lockID, environment)
		return nil
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	existingEnvLock.Deleted = true
	err = h.DBWriteEnvironmentLockInternal(ctx, tx, *existingEnvLock, previousVersion)

	if err != nil {
		return fmt.Errorf("could not delete environment lock from DB. Error: %w\n", err)
	}
	return nil
}

type AllEnvLocksJson struct {
	EnvLocks []string `json:"envLocks"`
}

type AllEnvLocksRow struct {
	Version     int64
	Created     time.Time
	Environment string
	Data        string
}

type AllEnvLocksGo struct {
	Version     int64
	Created     time.Time
	Environment string
	AllEnvLocksJson
}

type QueuedDeployment struct {
	EslVersion EslVersion
	Created    time.Time
	Env        string
	App        string
	Version    *int64
}

func (h *DBHandler) DBSelectAnyDeploymentAttempt(ctx context.Context, tx *sql.Tx) (*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyDeploymentAttempt")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAnyDeploymentAttempt: no transaction provided")
	}

	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts ORDER BY eslVersion DESC LIMIT 1;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery)
	return h.processDeploymentAttemptsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectDeploymentAttemptHistory(ctx context.Context, tx *sql.Tx, environmentName, appName string, limit int) ([]QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentAttemptHistory")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectDeploymentAttemptHistory: no transaction provided")
	}

	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts WHERE envName=? AND appName=? ORDER BY eslVersion DESC LIMIT ?;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery,
		environmentName,
		appName, limit)

	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	queuedDeployments := make([]QueuedDeployment, 0)
	for rows.Next() {
		row, err := h.processSingleDeploymentAttemptsRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		queuedDeployments = append(queuedDeployments, QueuedDeployment{
			EslVersion: row.EslVersion,
			Created:    row.Created,
			Env:        row.Env,
			App:        row.App,
			Version:    row.Version,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return queuedDeployments, nil
}

func (h *DBHandler) DBSelectLatestDeploymentAttempt(ctx context.Context, tx *sql.Tx, environmentName, appName string) (*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttempt")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectLatestDeploymentAttempt: no transaction provided")
	}
	insertQuery := h.AdaptQuery(
		"SELECT eslVersion, created, envName, appName, queuedReleaseVersion FROM deployment_attempts WHERE envName=? AND appName=? ORDER BY eslVersion DESC LIMIT 1;")

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery,
		environmentName,
		appName)
	return h.processDeploymentAttemptsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectLatestDeploymentAttemptOfAllApps(ctx context.Context, tx *sql.Tx, environmentName string) ([]*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttemptOfAllApps")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectLatestDeploymentAttemptOfAllApps: no transaction provided")
	}
	query := h.AdaptQuery(
		`
	SELECT DISTINCT
		deployment_attempts.eslversion,
		deployment_attempts.created,
		deployment_attempts.envName,
		deployment_attempts.appName,
		deployment_attempts.queuedReleaseVersion
	FROM (
		SELECT
			MAX(eslversion) AS latestRelease,
			appname,
			envName
		FROM
			"deployment_attempts"
		GROUP BY
			envname, appname) AS latest
	JOIN
		deployment_attempts AS deployment_attempts 
	ON
		latest.latestRelease=deployment_attempts.eslVersion
		AND latest.envName=deployment_attempts.envName
		AND latest.appname=deployment_attempts.appname
	WHERE deployment_attempts.envName=?
	ORDER BY deployment_attempts.eslversion DESC;
	`)
	span.SetTag("query", query)
	rows, err := tx.QueryContext(
		ctx,
		query,
		environmentName)
	return h.processDeploymentAttemptsRows(ctx, rows, err)
}

func (h *DBHandler) DBSelectLatestDeploymentAttemptOnAllEnvironments(ctx context.Context, tx *sql.Tx, appName string) ([]*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttemptOnAllEnvironments")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectLatestDeploymentAttemptOnAllEnvironments: no transaction provided")
	}
	query := h.AdaptQuery(
		`
	SELECT DISTINCT
		deployment_attempts.eslversion,
		deployment_attempts.created,
		deployment_attempts.envname,
		deployment_attempts.appname,
		deployment_attempts.queuedReleaseVersion
	FROM (
		SELECT
			MAX(eslversion) AS latestRelease,
			appname,
			envname
		FROM
			"deployment_attempts"
		GROUP BY
			envname, appname) AS latest
	JOIN
		deployment_attempts AS deployment_attempts
	ON
		latest.latestRelease=deployment_attempts.eslVersion
		AND latest.envname=deployment_attempts.envname
		AND latest.appname=deployment_attempts.appname
	WHERE deployment_attempts.appname=?
	ORDER BY deployment_attempts.eslversion DESC;
	`)
	span.SetTag("query", query)
	rows, err := tx.QueryContext(
		ctx,
		query,
		appName)
	return h.processDeploymentAttemptsRows(ctx, rows, err)
}

func (h *DBHandler) DBWriteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName, appName string, version *int64) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentAttempt")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteDeploymentAttempt: no transaction provided")
	}
	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		EslVersion: 0,
		Created:    time.Time{},
		Env:        envName,
		App:        appName,
		Version:    version,
	})
}

func (h *DBHandler) DBDeleteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName, appName string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentAttempt")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteDeploymentAttempt: no transaction provided")
	}
	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		EslVersion: 0,
		Created:    time.Time{},
		Env:        envName,
		App:        appName,
		Version:    nil,
	})
}

func (h *DBHandler) dbWriteDeploymentAttemptInternal(ctx context.Context, tx *sql.Tx, deployment *QueuedDeployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "dbWriteDeploymentAttemptInternal")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("dbWriteDeploymentAttemptInternal: no transaction provided")
	}
	latestDeployment, err := h.DBSelectLatestDeploymentAttempt(ctx, tx, deployment.Env, deployment.App)

	if err != nil {
		return fmt.Errorf("Could not get latest deployment attempt from deployments table")
	}
	var previousEslVersion EslVersion

	if latestDeployment == nil {
		previousEslVersion = 0
	} else {
		previousEslVersion = latestDeployment.EslVersion
	}
	nullVersion := NewNullInt(deployment.Version)

	insertQuery := h.AdaptQuery(
		"INSERT INTO deployment_attempts (eslVersion, created, envName, appName, queuedReleaseVersion) VALUES (?, ?, ?, ?, ?);")
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("dbWriteDeploymentAttemptInternal unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		*now,
		deployment.Env,
		deployment.App,
		nullVersion)

	if err != nil {
		return fmt.Errorf("could not write deployment attempts table in DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) processDeploymentAttemptsRows(ctx context.Context, rows *sql.Rows, err error) ([]*QueuedDeployment, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var results []*QueuedDeployment
	for rows.Next() {
		row, err := h.processSingleDeploymentAttemptsRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (h *DBHandler) processDeploymentAttemptsRow(ctx context.Context, rows *sql.Rows, err error) (*QueuedDeployment, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var row *QueuedDeployment
	if rows.Next() {
		row, err = h.processSingleDeploymentAttemptsRow(ctx, rows)
		if err != nil {
			return nil, err
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// processSingleDeploymentAttemptsRow only processes the row. It assumes that there is an element ready to be processed in rows.
func (h *DBHandler) processSingleDeploymentAttemptsRow(ctx context.Context, rows *sql.Rows) (*QueuedDeployment, error) {
	//exhaustruct:ignore
	var row = QueuedDeployment{}
	var releaseVersion sql.NullInt64

	err := rows.Scan(&row.EslVersion, &row.Created, &row.Env, &row.App, &releaseVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning deployment attempts row from DB. Error: %w\n", err)
	}

	if releaseVersion.Valid {
		row.Version = &releaseVersion.Int64
	}
	return &row, nil

}

func (h *DBHandler) RunCustomMigrationEnvironments(ctx context.Context, getAllEnvironmentsFun GetAllEnvironmentsFun) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvironments")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		needsMigrating, err := h.needsEnvironmentsMigrations(ctx, transaction)
		if err != nil {
			return err
		}
		if !needsMigrating {
			return nil
		}
		allEnvironments, err := getAllEnvironmentsFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get environments, error: %w", err)
		}

		allEnvsApps, err := h.FindEnvsAppsFromReleases(ctx, transaction)
		if err != nil {
			return err
		}
		for envName, config := range allEnvironments {
			if allEnvsApps[envName] == nil {
				allEnvsApps[envName] = make([]string, 0)
			}
			err = h.DBWriteEnvironment(ctx, transaction, envName, config, allEnvsApps[envName])
			if err != nil {
				return fmt.Errorf("unable to write manifest for environment %s to the database, error: %w", envName, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) needsEnvironmentsMigrations(ctx context.Context, transaction *sql.Tx) (bool, error) {
	log := logger.FromContext(ctx).Sugar()

	arbitraryAllEnvsRow, err := h.DBSelectAnyEnvironment(ctx, transaction)

	if err != nil {
		return true, err
	}
	if arbitraryAllEnvsRow != nil {
		log.Infof("custom migration for environments already ran because row (%v) was found, skipping custom migration", arbitraryAllEnvsRow)
		return false, nil
	}
	return true, nil
}

func (h *DBHandler) RunCustomMigrationEnvironmentApplications(ctx context.Context) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvironmentApplications")
	defer span.Finish()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		allEnvironments, err := h.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get environments, error: %w", err)
		}
		var allEnvsApps map[string][]string
		for _, envName := range allEnvironments {
			env, err := h.DBSelectEnvironment(ctx, transaction, envName)
			if err != nil {
				return fmt.Errorf("could not get env: %s, error: %w", envName, err)
			}

			if env.Applications == nil || len(env.Applications) == 0 {
				if allEnvsApps == nil {
					allEnvsApps, err = h.FindEnvsAppsFromReleases(ctx, transaction)
					if err != nil {
						return fmt.Errorf("could not find all applications of all environments, error: %w", err)
					}
				}
				if allEnvsApps[envName] == nil {
					allEnvsApps[envName] = make([]string, 0)
				}
				err = h.DBWriteEnvironment(ctx, transaction, envName, env.Config, allEnvsApps[envName])
				if err != nil {
					return fmt.Errorf("unable to write manifest for environment %s to the database, error: %w", envName, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) FindEnvsAppsFromReleases(ctx context.Context, tx *sql.Tx) (map[string][]string, error) {
	envsApps := make(map[string][]string)
	releases, err := h.DBSelectAllManifestsForAllReleases(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("could not get all environments for all releases, error: %w", err)
	}
	for app, versionEnvs := range releases {
		envSet := make(map[string]struct{})
		for _, envs := range versionEnvs {
			for _, env := range envs {
				envSet[env] = struct{}{}
			}
		}
		for env := range envSet {
			envsApps[env] = append(envsApps[env], app)
		}
	}
	return envsApps, nil
}

func (h *DBHandler) RunCustomMigrationReleaseEnvironments(ctx context.Context) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationReleaseEnvironments")
	defer span.Finish()
	for {
		shouldContinueMigration := true
		err := h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			releasesWithoutEnvironments, err := h.DBSelectReleasesWithoutEnvironments(ctx, transaction)
			if len(releasesWithoutEnvironments) == 0 {
				shouldContinueMigration = false
				return nil
			}
			if err != nil {
				return fmt.Errorf("could not get releases without environments, error: %w", err)
			}
			logger.FromContext(ctx).Sugar().Infof("updating %d releases environments", len(releasesWithoutEnvironments))
			for _, release := range releasesWithoutEnvironments {
				err = h.DBUpdateOrCreateRelease(ctx, transaction, *release)
				if err != nil {
					return fmt.Errorf("could not insert release, error: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if !shouldContinueMigration {
			break
		}
	}
	return nil
}

func (h *DBHandler) DBWriteFailedEslEvent(ctx context.Context, tx *sql.Tx, table string, eslEvent *EslFailedEventRow) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteFailedEslEvent")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteFailedEslEvent: no transaction provided")
	}

	insertQuery := h.AdaptQuery(fmt.Sprintf("INSERT INTO %s (created, event_type, json, reason, transformerEslVersion)  VALUES (?, ?, ?, ?, ?);", table))
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteFailedEslEvent unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		eslEvent.EventType,
		eslEvent.EventJson,
		eslEvent.Reason,
		eslEvent.TransformerEslVersion)

	if err != nil {
		return fmt.Errorf("could not write failed esl event into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteFailedEslEvent(ctx context.Context, tx *sql.Tx, eslEvent *EslFailedEventRow) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBDeleteFailedEslEvent")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBDeleteFailedEslEvent: no transaction provided"))
	}

	deleteQuery := h.AdaptQuery("DELETE FROM event_sourcing_light_failed WHERE transformereslversion=?;")
	span.SetTag("query", deleteQuery)
	_, err := tx.ExecContext(
		ctx,
		deleteQuery,
		eslEvent.TransformerEslVersion)

	if err != nil {
		return onErr(fmt.Errorf("could not delete failed esl event from DB. Error: %w\n", err))
	}
	return nil
}

func (h *DBHandler) DBReadLastFailedEslEvents(ctx context.Context, tx *sql.Tx, pageSize, pageNumber int) ([]*EslFailedEventRow, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadlastFailedEslEvents")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadlastFailedEslEvents: no transaction provided")
	}

	query := h.AdaptQuery("SELECT created, event_type, json, reason, transformerEslVersion FROM event_sourcing_light_failed ORDER BY created ASC LIMIT (?) OFFSET (?);")
	span.SetTag("query", query)
	rows, err := tx.QueryContext(ctx, query, pageSize+1, pageNumber*pageSize)
	if err != nil {
		return nil, fmt.Errorf("could not read failed events from DB. Error: %w\n", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	failedEsls := make([]*EslFailedEventRow, 0)

	for rows.Next() {
		row := &EslFailedEventRow{
			EslVersion:            0, //No esl version for currently failed events
			Created:               time.Unix(0, 0),
			EventType:             "",
			EventJson:             "",
			Reason:                "",
			TransformerEslVersion: 0,
		}
		err := rows.Scan(&row.Created, &row.EventType, &row.EventJson, &row.Reason, &row.TransformerEslVersion)
		if err != nil {
			return nil, fmt.Errorf("could not read failed events from DB. Error: %w\n", err)
		}
		failedEsls = append(failedEsls, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("could not close rows. Error: %w\n", err)
	}

	return failedEsls, nil
}

func (h *DBHandler) DBReadEslFailedEventFromEslVersion(ctx context.Context, tx *sql.Tx, eslVersion uint64) (*EslFailedEventRow, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadEslFailedEventFromTransformerId")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadEslFailedEventFromTransformerId: no transaction provided")
	}

	query := h.AdaptQuery(
		`SELECT created, event_type, json, reason, transformerEslVersion
	 FROM event_sourcing_light_failed WHERE transformerEslVersion=? ORDER BY created DESC LIMIT 1;`)
	span.SetTag("query", query)
	rows, err := tx.QueryContext(ctx, query, eslVersion)
	if err != nil {
		return nil, fmt.Errorf("could not read failed events from DB. Error: %w\n", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var row *EslFailedEventRow

	if rows.Next() {
		row = &EslFailedEventRow{
			EslVersion:            0,
			Created:               time.Unix(0, 0),
			EventType:             "",
			EventJson:             "",
			Reason:                "",
			TransformerEslVersion: 0,
		}
		err := rows.Scan(&row.Created, &row.EventType, &row.EventJson, &row.Reason, &row.TransformerEslVersion)
		if err != nil {
			return nil, fmt.Errorf("could not read failed events from DB. Error: %w\n", err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("could not close rows. Error: %w\n", err)
	}

	return row, nil
}

func (h *DBHandler) DBReadLastEslEvents(ctx context.Context, tx *sql.Tx, limit int) ([]*EslEventRow, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBReadLastEslEvents")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBReadlastFailedEslEvents: no transaction provided"))
	}

	query := h.AdaptQuery("SELECT eslVersion, created, event_type, json FROM event_sourcing_light ORDER BY eslVersion DESC LIMIT ?;")
	span.SetTag("query", query)
	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, onErr(fmt.Errorf("could not read failed events from DB. Error: %w\n", err))
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	failedEsls := make([]*EslEventRow, 0)

	for rows.Next() {
		row := &EslEventRow{
			EslVersion: 0,
			Created:    time.Unix(0, 0),
			EventType:  "",
			EventJson:  "",
		}
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			return nil, onErr(fmt.Errorf("could not read failed events from DB. Error: %w\n", err))
		}
		failedEsls = append(failedEsls, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, onErr(fmt.Errorf("could not close rows. Error: %w\n", err))
	}

	return failedEsls, nil
}

func (h *DBHandler) DBInsertNewFailedESLEvent(ctx context.Context, tx *sql.Tx, eslEvent *EslFailedEventRow) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBInsertNewFailedESLEvent")
	defer span.Finish()
	err := h.DBWriteFailedEslEvent(ctx, tx, "event_sourcing_light_failed", eslEvent)
	if err != nil {
		return onErr(err)
	}
	err = h.DBWriteFailedEslEvent(ctx, tx, "event_sourcing_light_failed_history", eslEvent)
	if err != nil {
		return onErr(err)
	}
	return nil
}

func (h *DBHandler) DBSkipFailedEslEvent(ctx context.Context, tx *sql.Tx, transformerEslVersion TransformerID) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBSkipFailedEslEvent")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBReadlastFailedEslEvents: no transaction provided"))
	}

	query := h.AdaptQuery("DELETE FROM event_sourcing_light_failed WHERE transformerEslVersion = ?;")
	span.SetTag("query", query)

	result, err := tx.ExecContext(ctx, query, transformerEslVersion)

	if result == nil {
		return onErr(grpc.FailedPrecondition(ctx, fmt.Errorf("could not find failed esl event where transformer esl version is %d", transformerEslVersion)))
	}
	if err != nil {
		return onErr(fmt.Errorf("could not write to cutoff table from DB. Error: %w\n", err))
	}
	return nil
}

func (h *DBHandler) DBReadTransactionTimestamp(ctx context.Context, tx *sql.Tx) (*time.Time, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("attempting to read transaction timestamp without a transaction")
	}

	var query string
	if h.DriverName == "sqlite3" { //Testing purposes
		query = "select CURRENT_TIMESTAMP as now;"
	} else {
		query = "select now();"
	}
	rows, err := tx.QueryContext(
		ctx,
		query,
	)

	if err != nil {
		return nil, fmt.Errorf("DBReadTransactionTimestamp error executing query: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	var now time.Time

	if rows.Next() {
		if h.DriverName == "sqlite3" { //Testing purposes
			var nowString string
			err = rows.Scan(&nowString)
			if err != nil {
				return nil, fmt.Errorf("DBReadTransactionTimestamp error scanning database response query: %w", err)
			}
			now, err = time.Parse(time.DateTime, nowString)
			if err != nil {
				return nil, fmt.Errorf("DBReadTransactionTimestamp error converting: %w", err)
			}
		} else {
			err = rows.Scan(&now)
			if err != nil {
				return nil, fmt.Errorf("DBReadTransactionTimestamp error scanning database response query: %w", err)
			}
		}

		now = now.UTC()
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("could not close rows. Error: %w\n", err)
	}
	return &now, nil
}

func (h *DBHandler) DBWriteCommitTransactionTimestamp(ctx context.Context, tx *sql.Tx, commitHash string, timestamp time.Time) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCommitTransactionTimestamp")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the commit_transaction_timestamps table without a transaction")
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO commit_transaction_timestamps (commitHash, transactionTimestamp) VALUES (?, ?);",
	)

	span.SetTag("query", insertQuery)
	span.SetTag("commitHash", commitHash)
	span.SetTag("timestamp", timestamp)
	_, err := tx.Exec(
		insertQuery,
		commitHash,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("DBWriteCommitTransactionTimestamp error executing query: %w", err)
	}
	return nil
}

func (h *DBHandler) DBReadCommitHashTransactionTimestamp(ctx context.Context, tx *sql.Tx, commitHash string) (*time.Time, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCommitTransactionTimestamp")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("attempting to read to the commit_transaction_timestamps table without a transaction")
	}

	insertQuery := h.AdaptQuery(
		"SELECT transactionTimestamp " +
			"FROM commit_transaction_timestamps " +
			"WHERE commitHash=?;",
	)

	span.SetTag("query", insertQuery)
	rows, err := tx.QueryContext(
		ctx,
		insertQuery,
		commitHash,
	)
	if err != nil {
		return nil, fmt.Errorf("DBReadCommitHashTransactionTimestamp error executing query: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var timestamp *time.Time

	if rows.Next() {
		timestamp = &time.Time{}

		err = rows.Scan(timestamp)
		if err != nil {
			return nil, fmt.Errorf("DBReadTransactionTimestamp error scanning database response query: %w", err)
		}

		*timestamp = timestamp.UTC()
	} else {
		timestamp = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("could not close rows. Error: %w\n", err)
	}
	return timestamp, nil
}
