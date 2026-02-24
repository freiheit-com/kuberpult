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
	"math"
	"slices"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	psql "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
	"github.com/onokonem/sillyQueueServer/timeuuid"
	ddsql "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
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

	DatadogEnabled     bool
	DatadogServiceName string
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
	MaxInsertBatchSize       = 1000
	MaxDeleteBatchSize       = 20
)

func (h *DBHandler) ShouldUseEslTable() bool {
	return h != nil
}

// ShouldUseOtherTables returns true if the db is enabled and WriteEslOnly=false
// ShouldUseOtherTables should never be used in the manifest-repo-export-service.
func (h *DBHandler) ShouldUseOtherTables() bool {
	return h != nil && !h.WriteEslOnly
}

func Connect(ctx context.Context, cfg DBConfig) (*DBHandler, error) {
	if cfg.DriverName != "postgres" {
		return nil, fmt.Errorf("WRONG TEST SETUP without postgres")
	}
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

		var dbPool *sql.DB
		var err error
		if cfg.DatadogEnabled {
			ddsql.Register(cfg.DriverName, pq.Driver{})
			dbPool, err = ddsql.Open(cfg.DriverName,
				dbURI,
				ddsql.WithServiceName(cfg.DatadogServiceName),
				ddsql.WithDBMPropagation(tracer.DBMPropagationModeFull),
			)
			if err != nil {
				return nil, fmt.Errorf("sql.Open with datadog: %w", err)
			}
		} else {
			dbPool, err = sql.Open(cfg.DriverName, dbURI)
			if err != nil {
				return nil, fmt.Errorf("sql.Open: %w", err)
			}
		}
		dbPool.SetConnMaxLifetime(5 * time.Minute)
		dbPool.SetMaxOpenConns(int(cfg.MaxOpenConnections))
		dbPool.SetMaxIdleConns(int(cfg.MaxIdleConnections))
		return dbPool, nil
	}
	return nil, fmt.Errorf("driver: only postgres is supported, but not '%s'", cfg.DriverName)
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
		d := time.Second * 2
		l.Warnf("could not connect to db, will try again in %v for %d more times, error: %v", d, i, err)
		time.Sleep(d)
	}
	return nil, nil, err

}

func GetConnectionAndDriver(cfg DBConfig) (*sql.DB, database.Driver, error) {
	db, err := GetDBConnection(cfg)
	if err != nil {
		return nil, nil, err
	}
	if cfg.DriverName != "postgres" {
		return nil, nil, fmt.Errorf("driver: '%s' not supported. Supported: postgres", cfg.DriverName)
	}
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
}

func (h *DBHandler) getMigrationHandler() (*migrate.Migrate, error) {
	if h.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, h.DbName, *h.DBDriver)
	}
	return nil, fmt.Errorf("driver: '%s' not supported. Supported: postgres", h.DriverName)
}

func RunDBMigrations(ctx context.Context, cfg DBConfig) error {
	d, err := Connect(ctx, cfg)
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w", err)
	}
	defer func() { _ = d.DB.Close() }()

	m, err := d.getMigrationHandler()

	if err != nil {
		return fmt.Errorf("error creating migration instance. Error: %w", err)
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error running DB migrations. Error: %w", err)
		}
	}
	return nil
}

func (h *DBHandler) AdaptQuery(query string) string {
	if h.DriverName == "postgres" {
		return SqliteToPostgresQuery(query)
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
		return fmt.Errorf("row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return fmt.Errorf("row has error: %v", err)
	}
	return nil
}

func closeRowsAndLog(rows *sql.Rows, ctx context.Context, prefix string) {
	err := rows.Close()
	if err != nil {
		logger.FromContext(ctx).Sugar().Warnf("%s: rows could not be closed: %v", prefix, err)
	}
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
	EvtExtendAAEnvironment              EventType = "ExtendAAEnvironment"
	EvtDeleteAAEnvironmentConfig        EventType = "EvtDeleteAAEnvironmentConfig"
)

// ESL EVENTS

type ESLMetadata struct {
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
}

// DBWriteEslEventInternal writes one event to the event-sourcing-light table, taking arbitrary data as input
func (h *DBHandler) DBWriteEslEventInternal(ctx context.Context, eventType EventType, tx *sql.Tx, data interface{}, metadata ESLMetadata) (err error) {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (created, event_type, json, trace_id, span_id)  VALUES (?, ?, ?, ?, ?);")

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEslEventInternal unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		eventType,
		jsonToInsert,
		span.Context().TraceID(),
		span.Context().SpanID())

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBWriteEslEventWithJson(ctx context.Context, tx *sql.Tx, eventType EventType, data string) (err error) {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
		return fmt.Errorf("could not write internal esl event into DB. Error: %w", err)
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
	TraceId    *uint64
	SpanId     *uint64
}

type EslFailedEventRow struct {
	EslVersion            EslVersion
	Created               time.Time
	EventType             EventType
	EventJson             string
	Reason                string
	TransformerEslVersion EslVersion
}

// DBReadEslEventInternal returns either the first or the last row of the esl table
func (h *DBHandler) DBReadEslEventInternal(ctx context.Context, tx *sql.Tx, firstRow bool) (_ *EslEventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadEslEventInternal")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	sort := "DESC"
	if firstRow {
		sort = "ASC"
	}
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslVersion, created, event_type, json, trace_id, span_id FROM event_sourcing_light ORDER BY eslVersion %s LIMIT 1;", sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBReadEslEventInternal")
	zeroTrace := uint64(0)
	zeroSpan := uint64(0)
	var row = &EslEventRow{
		EslVersion: 0,
		Created:    time.Unix(0, 0),
		EventType:  "",
		EventJson:  "",
		TraceId:    &zeroTrace,
		SpanId:     &zeroSpan,
	}
	if rows.Next() {
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson, &row.TraceId, &row.SpanId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning event_sourcing_light row from DB. Error: %w", err)
		}
	} else {
		row = nil
	}
	return row, nil
}

// DBReadEslEventLaterThan returns the first row of the esl table that has an eslVersion > the given eslVersion
func (h *DBHandler) DBReadEslEventLaterThan(ctx context.Context, tx *sql.Tx, eslVersion EslVersion) (_ *EslEventRow, err error) {
	sort := "ASC"
	selectQuery := h.AdaptQuery(fmt.Sprintf(`
		SELECT eslVersion, created, event_type, json, trace_id, span_id
		FROM event_sourcing_light
		WHERE eslVersion > (?)
		ORDER BY eslVersion %s
		LIMIT 1;
		`, sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		eslVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBReadEslEventLaterThan")
	zeroTrace := uint64(0)
	zeroSpan := uint64(0)
	var row = &EslEventRow{
		EslVersion: 0,
		Created:    time.Unix(0, 0),
		EventType:  "",
		EventJson:  "",
		TraceId:    &zeroTrace,
		SpanId:     &zeroSpan,
	}
	if !rows.Next() {
		row = nil
	} else {
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson, &row.TraceId, &row.SpanId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("event_sourcing_light: Error scanning row from DB. Error: %w", err)
		}
	}
	return row, nil
}

func (h *DBHandler) DBReadEslEvent(ctx context.Context, transaction *sql.Tx, eslVersion *EslVersion) (*EslEventRow, error) {
	log := logger.FromContext(ctx).Sugar()
	if eslVersion == nil {
		log.Warnf("no cutoff found, starting at the beginning of time.")
		// no read cutoff yet, we have to start from the beginning
		esl, err := h.DBReadEslEventInternal(ctx, transaction, true)
		if err != nil {
			return nil, err
		}
		if esl == nil {
			log.Warnf("no esl events found")
			return nil, nil
		}
		return esl, nil
	} else {
		esl, err := h.DBReadEslEventLaterThan(ctx, transaction, *eslVersion)
		if err != nil {
			return nil, err
		}
		return esl, nil
	}
}
func (h *DBHandler) DBCountEslEventsNewer(ctx context.Context, tx *sql.Tx, eslVersion EslVersion) (_ uint64, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBCountEslEventsNewer")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	countQuery := h.AdaptQuery("SELECT COUNT(*) FROM event_sourcing_light WHERE eslVersion > ?;")
	rows, err := tx.QueryContext(
		ctx,
		countQuery,
		eslVersion,
	)
	if err != nil {
		return 0, fmt.Errorf("could not query event_sourcing_light table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBCountEslEventsNewer")
	if !rows.Next() {
		return 0, fmt.Errorf("could not get count from event_sourcing_light table from DB. Error: no row returned")
	}
	count := uint64(0)
	errScan := rows.Scan(&count)
	if errScan != nil {
		return 0, fmt.Errorf("error scanning event_sourcing_light row from DB. Error: %w", err)
	}
	return count, nil
}

func (h *DBHandler) WriteEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, eventuuid string, eventType event.EventType, sourceCommitHash string, eventJson []byte) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "WriteEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("eventType", eventType)
	span.SetTag("commitHash", sourceCommitHash)

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
		return fmt.Errorf("error inserting event information into DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBWriteNewReleaseEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, releaseVersion types.ReleaseNumbers, uuid, sourceCommitHash string, newReleaseEvent *event.NewRelease) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	version := uint64(0)
	if releaseVersion.Version != nil {
		version = *releaseVersion.Version
	}
	metadata := event.Metadata{
		Uuid:           uuid,
		EventType:      string(event.EventTypeNewRelease),
		ReleaseVersion: version,
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     newReleaseEvent,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling lock new release event to Json. Error: %v", err)
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
		return fmt.Errorf("error marshalling lock prevented deployment event to Json. Error: %v", err)
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
		return fmt.Errorf("error marshalling replacedBys event to Json. Error: %v", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeReplaceBy, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBWriteDeploymentEvent(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, uuid, sourceCommitHash string, deployment *event.Deployment) (err error) {
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
		return fmt.Errorf("error marshalling deployment event to Json. Error: %v", err)
	}
	return h.WriteEvent(ctx, transaction, transformerID, uuid, event.EventTypeDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBSelectAnyEvent(ctx context.Context, transaction *sql.Tx) (_ *EventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		ORDER BY timestamp DESC
		LIMIT 1;`,
	)
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query)
	return h.processSingleEventsRow(ctx, rows, err)
}

func (h *DBHandler) DBContainsMigrationCommitEvent(ctx context.Context, transaction *sql.Tx) (_ bool, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBContainsMigrationCommitEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		WHERE commitHash = (?)
		ORDER BY timestamp DESC
		LIMIT 1;`,
	)
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query, MigrationCommitEventHash)

	row, err := h.processSingleEventsRow(ctx, rows, err)

	if err != nil {
		return false, err
	}

	return row != nil, nil
}

func (h *DBHandler) DBSelectAllCommitEventsForTransformer(ctx context.Context, transaction *sql.Tx, transformerID TransformerID, eventType event.EventType, limit uint) (_ []event.DBEventGo, err error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllCommitEventsForTransformer: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllCommitEventsForTransformer")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		WHERE eventType = (?) AND transformereslVersion = (?)
		ORDER BY timestamp DESC
		LIMIT ?;`,
	)
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, string(eventType), transformerID, limit)
	if err != nil {
		return nil, fmt.Errorf("error querying commit_events. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "commit_events")
	var result []event.DBEventGo
	for rows.Next() {
		//exhaustruct:ignore
		var row = &EventRow{}
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning commit_events row from DB. Error: %w", err)
		}

		eventGo, err := event.UnMarshallEvent(row.EventType, row.EventJson)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshall commit event: %v", err)
		}
		result = append(result, eventGo)
	}
	span.SetTag("resultLength", len(result))
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v", err)
	}
	return result, nil
}

func (h *DBHandler) processSingleEventsRow(ctx context.Context, rows *sql.Rows, err error) (*EventRow, error) {
	if err != nil {
		return nil, fmt.Errorf("error querying commit_events. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "commit_events")
	//exhaustruct:ignore
	var row = &EventRow{}
	if rows.Next() {
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning commit_events row from DB. Error: %w", err)
		}
	} else {
		row = nil
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v", err)
	}
	return row, nil
}

func (h *DBHandler) DBSelectAllEventsForCommit(ctx context.Context, transaction *sql.Tx, commitHash string, pageNumber, pageSize uint64) (_ []EventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEventsForCommit")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	// NOTE: We add one so we know if there is more to load
	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		WHERE commitHash = (?)
		ORDER BY timestamp ASC
		LIMIT (?)
		OFFSET (?);`,
	)
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, commitHash, pageSize+1, pageNumber*pageSize)
	return processAllCommitEventRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectAllCommitEventsForTransformerID(ctx context.Context, transaction *sql.Tx, transformerID TransformerID) (_ []EventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllCommitEventsForTransformerID")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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

func (h *DBHandler) DBSelectAllLockPreventedEventsForTransformerID(ctx context.Context, transaction *sql.Tx, transformerID TransformerID) (_ []EventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllLockPreventedEventsForTransformerID")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	query := h.AdaptQuery(`
		SELECT uuid, timestamp, commitHash, eventType, json, transformereslVersion
		FROM commit_events
		WHERE transformereslVersion = (?) AND eventtype = (?)
		ORDER BY timestamp DESC
		LIMIT 100;`,
	)
	span.SetTag("query", query)

	rows, err := transaction.QueryContext(ctx, query, transformerID, string(event.EventTypeLockPreventedDeployment))
	return processAllCommitEventRow(ctx, rows, err)
}

func processAllCommitEventRow(ctx context.Context, rows *sql.Rows, err error) ([]EventRow, error) {
	if err != nil {
		return nil, fmt.Errorf("error querying commit_events. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "commit_events")
	var result []EventRow

	for rows.Next() {
		row, err := processSingleCommitEventRow(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, *row)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("commit_events: row has error: %v", err)
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
		return nil, fmt.Errorf("error scanning commit_events row from DB. Error: %w", err)
	}
	return &row, nil
}

type LockMetadata struct {
	CreatedByName     string
	CreatedByEmail    string
	Message           string
	CiLink            string
	CreatedAt         time.Time
	SuggestedLifeTime string
}

type LockDeletionMetadata struct {
	DeletedByUser  string
	DeletedByEmail string
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

	Manifests map[types.EnvName]string // keys: environment; value: manifest
}

type AllDeployments []Deployment
type AllEnvLocks map[string][]EnvironmentLock
type AllReleases map[uint64]ReleaseWithManifest // keys: releaseVersion; value: release with manifests

// WriteAllDeploymentsFun and other functions here are used during migration.
// They are supposed to read data from files in the manifest repo and write it to the databse,
// and therefore need to access the Database.
type WriteAllDeploymentsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllAppLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error

type AllAppLocks map[types.EnvName]map[string][]ApplicationLock // EnvName-> AppName -> []Locks
type AllTeamLocks map[types.EnvName]map[string][]TeamLock       // EnvName-> Team -> []Locks
type AllQueuedVersions map[types.EnvName]map[string]*int64      // EnvName-> AppName -> queuedVersion
type AllCommitEvents map[string][]event.DBEventGo               // CommitId -> uuid -> Event

type WriteAllEnvLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllTeamLocksFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllReleasesFun = func(ctx context.Context, transaction *sql.Tx, app types.AppName, dbHandler *DBHandler) error
type WriteAllQueuedVersionsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type WriteAllEventsFun = func(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler) error
type FixReleasesTimestampFun = func(ctx context.Context, transaction *sql.Tx, app types.AppName, dbHandler *DBHandler) error

// GetAllAppsFun returns a map where the Key is an app name, and the value is a team name of that app
type GetAllAppsFun = func() (map[types.AppName]string, error)

// return value is a map from environment name to environment config
type GetAllEnvironmentsFun = func(ctx context.Context) (map[types.EnvName]config.EnvironmentConfig, error)

func (h *DBHandler) RunGit2DBMigrations(
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
) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.RunCustomMigrationsEventSourcingLight(ctx)
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

func (h *DBHandler) RunCustomMigrationReleases(ctx context.Context, getAllAppsFun GetAllAppsFun, writeAllReleasesFun WriteAllReleasesFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationReleases")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	hasRelease, err := h.DBHasAnyRelease(ctx, transaction, true)
	if err != nil {
		return true, err
	}
	if hasRelease {
		logger.FromContext(ctx).Sugar().Warnf("There are already deployments in the DB - skipping migrations")
	}
	return !hasRelease, nil
}

func (h *DBHandler) RunCustomMigrationReleasesTimestamp(ctx context.Context, getAllAppsFun GetAllAppsFun, fixReleasesTimestampFun FixReleasesTimestampFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationReleases")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	var allAppsMap map[types.AppName]string

	err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		allAppsMap, err = getAllAppsFun()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	for app := range allAppsMap {
		err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			l := logger.FromContext(ctx).Sugar()
			l.Infof("processing app %s ...", app)
			err := fixReleasesTimestampFun(ctx, transaction, app, h)
			if err != nil {
				return fmt.Errorf("could not migrate releases to database: %v", err)
			}
			l.Infof("done with app %s", app)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationDeployments(ctx context.Context, getAllDeploymentsFun WriteAllDeploymentsFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationDeployments")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	hasDeployment, err := h.DBHasAnyDeployment(ctx, transaction)
	if err != nil {
		return true, err
	}
	if hasDeployment {
		l.Warnf("There are already deployments in the DB - skipping migrations")
	}
	return !hasDeployment, nil
}

type EventRow struct {
	Uuid          string
	Timestamp     time.Time
	CommitHash    string
	EventType     event.EventType
	EventJson     string
	TransformerID TransformerID
}

func (h *DBHandler) RunCustomMigrationEnvLocks(ctx context.Context, writeAllEnvLocksFun WriteAllEnvLocksFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvLocks")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	hasEnvLock, err := h.DBHasAnyActiveEnvLock(ctx, transaction)
	if err != nil {
		return true, err
	}
	if hasEnvLock {
		l.Infof("There are already environment locks in the DB - skipping migrations")
	}
	return !hasEnvLock, nil
}

func (h *DBHandler) RunCustomMigrationAppLocks(ctx context.Context, writeAllAppLocksFun WriteAllAppLocksFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationAppLocks")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	hasAppLock, err := h.DBHasAnyActiveAppLock(ctx, transaction)
	if err != nil {
		return true, err
	}
	if hasAppLock {
		logger.FromContext(ctx).Sugar().Infof("There are already application locks in the DB - skipping migrations")
	}
	return !hasAppLock, nil
}

func (h *DBHandler) RunCustomMigrationTeamLocks(ctx context.Context, writeAllTeamLocksFun WriteAllTeamLocksFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationTeamLocks")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	hasTeamLock, err := h.DBHasAnyActiveTeamLock(ctx, transaction)
	if err != nil {
		return true, err
	}
	if hasTeamLock {
		logger.FromContext(ctx).Sugar().Infof("There are already team locks in the DB - skipping migrations")
	}
	return !hasTeamLock, nil
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
			return fmt.Errorf("error writing migration commit event to the database: %w", err)
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
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) (err error) {
		span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationsEventSourcingLight")
		defer func() {
			span.Finish(tracer.WithError(err))
		}()

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

func (h *DBHandler) DBWriteMigrationsTransformer(ctx context.Context, transaction *sql.Tx) (err error) {
	if h == nil {
		return nil
	}
	if transaction == nil {
		return fmt.Errorf("DBWriteMigrationsTransformer: no transaction provided")
	}

	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteMigrationsTransformer")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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

func (h *DBHandler) RunAllCustomMigrationsForApps(ctx context.Context, getAllAppsFun GetAllAppsFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunAllCustomMigrationsForApps")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
			return fmt.Errorf("could not perform apps table migration: %v", err)
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
func (h *DBHandler) runCustomMigrationApps(ctx context.Context, transaction *sql.Tx, appsMap *map[types.AppName]string) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "runCustomMigrationApps")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	for app, team := range *appsMap {
		err := h.DBInsertOrUpdateApplication(ctx, transaction, app, AppStateChangeMigrate, DBAppMetaData{Team: team})
		if err != nil {
			return fmt.Errorf("could not write dbApp %s: %v", app, err)
		}
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationEnvironments(ctx context.Context, getAllEnvironmentsFun GetAllEnvironmentsFun) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvironments")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
				allEnvsApps[envName] = make([]types.AppName, 0)
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

	hasEnv, err := h.DBHasAnyEnvironment(ctx, transaction)

	if err != nil {
		return true, err
	}
	if hasEnv {
		log.Infof("custom migration for environments already ran because row was found, skipping custom migration")
	}
	return !hasEnv, nil
}

func (h *DBHandler) RunCustomMigrationCleanGitSyncStatus(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationCleanGitSyncStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		return h.truncateGitSyncStatus(ctx, transaction)
	})
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationCleanOutdatedDeployments(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationCleanOutdatedDeployments")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	var orphanDeployments []Deployment
	err = h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		orphanDeployments, err = h.DBSelectAllOrphanDeployments(ctx, transaction)
		return err
	})
	if err != nil {
		return fmt.Errorf("could not get all orphan deployments: %w", err)
	}

	// delete the orphan deployments
	for deploymentBatch := range slices.Chunk(orphanDeployments, MaxDeleteBatchSize) {
		err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			for _, deployment := range deploymentBatch {
				err = h.DBDeleteDeployment(ctx, transaction, deployment.App, deployment.Env)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not delete deployments: %w", err)
		}
	}

	var allDeployments []Deployment
	err = h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		allDeployments, err = h.DBSelectAllDeployments(ctx, transaction, true)
		return err
	})
	if err != nil {
		return fmt.Errorf("could not get all deployments: %w", err)
	}

	// delete deployments with release versions that do not have manifests for the environment they are deployed to
	for deploymentBatch := range slices.Chunk(allDeployments, MaxDeleteBatchSize) {
		err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			for _, deployment := range deploymentBatch {
				release, err := h.DBSelectReleaseByVersion(ctx, transaction, deployment.App, deployment.ReleaseNumbers, false)
				if err != nil {
					return fmt.Errorf("could not fetch release %v for app %s: %w", deployment.ReleaseNumbers, deployment.App, err)
				}
				if release == nil { // release does not exist
					err = h.DBDeleteDeployment(ctx, transaction, deployment.App, deployment.Env)
					if err != nil {
						return fmt.Errorf("could not delete deployment for app %s in env %s: %w", deployment.App, deployment.Env, err)
					}
				} else if _, ok := release.Manifests.Manifests[deployment.Env]; !ok {
					err = h.DBDeleteDeployment(ctx, transaction, deployment.App, deployment.Env)
					if err != nil {
						return fmt.Errorf("could not delete deployment for app %s in env %s: %w", deployment.App, deployment.Env, err)
					}
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not delete deployments: %w", err)
		}
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationEnvironmentApplications(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationEnvironmentApplications")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		allEnvironments, err := h.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get environments, error: %w", err)
		}
		var allEnvsApps map[types.EnvName][]types.AppName
		for _, envName := range allEnvironments {
			env, err := h.DBSelectEnvironment(ctx, transaction, envName)
			if err != nil {
				return fmt.Errorf("could not get env: %s, error: %w", envName, err)
			}

			// We don't use environment applications column anymore, but for backward compatibility we keep it updated
			if len(env.Applications) == 0 {
				if allEnvsApps == nil {
					allEnvsApps, err = h.FindEnvsAppsFromReleases(ctx, transaction)
					if err != nil {
						return fmt.Errorf("could not find all applications of all environments, error: %w", err)
					}
				}
				if allEnvsApps[envName] == nil {
					allEnvsApps[envName] = make([]types.AppName, 0)
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

func (h *DBHandler) FindEnvsAppsFromReleases(ctx context.Context, tx *sql.Tx) (map[types.EnvName][]types.AppName, error) {
	envsApps := make(map[types.EnvName][]types.AppName)
	releases, err := h.DBSelectAllEnvironmentsForAllReleases(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("could not get all environments for all releases, error: %w", err)
	}
	for app, versionEnvs := range releases {
		envSet := make(map[types.EnvName]struct{})
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

func (h *DBHandler) RunCustomMigrationReleaseEnvironments(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationReleaseEnvironments")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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

func (h *DBHandler) DBWriteFailedEslEvent(ctx context.Context, tx *sql.Tx, table string, eslEvent *EslFailedEventRow) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteFailedEslEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		return fmt.Errorf("could not write failed esl event into DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteFailedEslEvent(ctx context.Context, tx *sql.Tx, eslEvent *EslFailedEventRow) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteFailedEslEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteFailedEslEvent: no transaction provided")
	}

	deleteQuery := h.AdaptQuery("DELETE FROM event_sourcing_light_failed WHERE transformereslversion=?;")
	span.SetTag("query", deleteQuery)
	_, err = tx.ExecContext(
		ctx,
		deleteQuery,
		eslEvent.TransformerEslVersion)

	if err != nil {
		return fmt.Errorf("could not delete failed esl event from DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBReadLastFailedEslEvents(ctx context.Context, tx *sql.Tx, pageSize, pageNumber int) (_ []*EslFailedEventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadLastFailedEslEvents")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		return nil, fmt.Errorf("could not read failed events from DB. Error: %w", err)
	}

	defer closeRowsAndLog(rows, ctx, "DBReadLastFailedEslEvents")
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
			return nil, fmt.Errorf("could not read failed events from DB. Error: %w", err)
		}
		failedEsls = append(failedEsls, row)
	}
	return failedEsls, nil
}

func (h *DBHandler) DBReadEslFailedEventFromEslVersion(ctx context.Context, tx *sql.Tx, eslVersion uint64) (_ *EslFailedEventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadEslFailedEventFromTransformerId")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		return nil, fmt.Errorf("could not read failed events from DB. Error: %w", err)
	}

	defer closeRowsAndLog(rows, ctx, "DBReadEslFailedEventFromTransformerId")

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
			return nil, fmt.Errorf("could not read failed events from DB. Error: %w", err)
		}
	}
	return row, nil
}

func (h *DBHandler) DBReadLastEslEvents(ctx context.Context, tx *sql.Tx, limit int) (_ []*EslEventRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadLastEslEvents")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadlastFailedEslEvents: no transaction provided")
	}

	query := h.AdaptQuery("SELECT eslVersion, created, event_type, json, trace_id, span_id FROM event_sourcing_light ORDER BY eslVersion DESC LIMIT ?;")
	span.SetTag("query", query)
	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("could not read last events from DB. Error: %w", err)
	}

	defer closeRowsAndLog(rows, ctx, "DBReadLastEslEvents")
	failedEsls := make([]*EslEventRow, 0)

	for rows.Next() {
		zeroTrace := uint64(0)
		zeroSpan := uint64(0)
		row := &EslEventRow{
			EslVersion: 0,
			Created:    time.Unix(0, 0),
			EventType:  "",
			EventJson:  "",
			TraceId:    &zeroTrace,
			SpanId:     &zeroSpan,
		}
		err := rows.Scan(&row.EslVersion, &row.Created, &row.EventType, &row.EventJson, &row.TraceId, &row.SpanId)
		if err != nil {
			return nil, fmt.Errorf("could not read failed events from DB. Error: %w", err)
		}
		failedEsls = append(failedEsls, row)
	}
	return failedEsls, nil
}

func (h *DBHandler) DBInsertNewFailedESLEvent(ctx context.Context, tx *sql.Tx, eslEvent *EslFailedEventRow) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertNewFailedESLEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.DBWriteFailedEslEvent(ctx, tx, "event_sourcing_light_failed", eslEvent)
	if err != nil {
		return err
	}
	err = h.DBWriteFailedEslEvent(ctx, tx, "event_sourcing_light_failed_history", eslEvent)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBSkipFailedEslEvent(ctx context.Context, tx *sql.Tx, transformerEslVersion TransformerID) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSkipFailedEslEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBReadlastFailedEslEvents: no transaction provided")
	}

	query := h.AdaptQuery("DELETE FROM event_sourcing_light_failed WHERE transformerEslVersion = ?;")
	span.SetTag("query", query)

	result, err := tx.ExecContext(ctx, query, transformerEslVersion)

	if result == nil {
		return grpc.FailedPrecondition(ctx, fmt.Errorf("could not find failed esl event where transformer esl version is %d", transformerEslVersion))
	}
	if err != nil {
		return fmt.Errorf("could not write to cutoff table from DB. Error: %w", err)
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

	var query = "select now();"
	rows, err := tx.QueryContext(
		ctx,
		query,
	)

	if err != nil {
		return nil, fmt.Errorf("DBReadTransactionTimestamp error executing query: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBReadTransactionTimestamp")
	var now time.Time

	if rows.Next() {
		err = rows.Scan(&now)
		if err != nil {
			return nil, fmt.Errorf("DBReadTransactionTimestamp error scanning database response query: %w", err)
		}

		now = now.UTC()
	}
	return &now, nil
}

func (h *DBHandler) DBWriteCommitTransactionTimestamp(ctx context.Context, tx *sql.Tx, commitHash string, timestamp time.Time) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCommitTransactionTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	_, err = tx.Exec(
		insertQuery,
		commitHash,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("DBWriteCommitTransactionTimestamp error executing query: %w", err)
	}
	return nil
}

func (h *DBHandler) DBUpdateCommitTransactionTimestamp(ctx context.Context, tx *sql.Tx, commitHash string, timestamp time.Time) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBUpdateCommitTransactionTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the commit_transaction_timestamps table without a transaction")
	}

	insertQuery := h.AdaptQuery(
		"UPDATE commit_transaction_timestamps SET transactionTimestamp=? WHERE commitHash=?;",
	)

	_, err = tx.Exec(
		insertQuery,
		timestamp,
		commitHash,
	)
	if err != nil {
		return fmt.Errorf("DBUpdateCommitTransactionTimestamp error executing query: %w", err)
	}
	return nil
}

func (h *DBHandler) DBReadCommitHashTransactionTimestamp(ctx context.Context, tx *sql.Tx, commitHash string) (_ *time.Time, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBReadCommitHashTransaction")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	defer closeRowsAndLog(rows, ctx, "DBReadCommitHashTransaction")

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
	return timestamp, nil
}

func (h *DBHandler) GetCurrentDelays(ctx context.Context, transaction *sql.Tx) (float64, uint64) {
	eslVersion, err := DBReadCutoff(h, ctx, transaction)
	if err != nil {
		return math.NaN(), 0
	}
	esl, err := h.DBReadEslEvent(ctx, transaction, eslVersion)
	if err != nil {
		return math.NaN(), 0
	}
	if esl == nil {
		return 0, 0
	}
	if esl.Created.IsZero() {
		return 0, 0
	}
	count, err := h.DBCountEslEventsNewer(ctx, transaction, esl.EslVersion)
	now := time.Now().UTC()
	diff := now.Sub(esl.Created).Seconds()
	if err != nil {
		return diff, 1
	}
	return diff, count
}
