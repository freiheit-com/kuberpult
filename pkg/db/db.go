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
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	uuid2 "github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/onokonem/sillyQueueServer/timeuuid"
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
	WriteEslOnly   bool
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

type EslId int64

type AppStateChange string

const (
	InitialEslId EslId = 1

	AppStateChangeMigrate AppStateChange = "AppStateChangeMigrate"
	AppStateChangeCreate  AppStateChange = "AppStateChangeCreate"
	AppStateChangeUpdate  AppStateChange = "AppStateChangeUpdate"
	AppStateChangeDelete  AppStateChange = "AppStateChangeDelete"
)

func (h *DBHandler) ShouldUseEslTable() bool {
	return h != nil
}

func (h *DBHandler) ShouldUseOtherTables() bool {
	return h != nil && !h.WriteEslOnly
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
		WriteEslOnly:   cfg.WriteEslOnly,
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

type DBFunction func(ctx context.Context, transaction *sql.Tx) error

func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

// WithTransaction opens a transaction, runs `f` and then calls either Commit or Rollback.
// Use this if the only thing to return from `f` is an error.
func (h *DBHandler) WithTransaction(ctx context.Context, f DBFunction) error {
	_, err := WithTransactionT(h, ctx, func(ctx context.Context, transaction *sql.Tx) (*interface{}, error) {
		err2 := f(ctx, transaction)
		if err2 != nil {
			return nil, err2
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

type DBFunctionT[T any] func(ctx context.Context, transaction *sql.Tx) (*T, error)

// WithTransactionT is the same as WithTransaction, but you can also return data, not just the error.
func WithTransactionT[T any](h *DBHandler, ctx context.Context, f DBFunctionT[T]) (*T, error) {
	res, err := WithTransactionMultipleEntriesT(h, ctx, func(ctx context.Context, transaction *sql.Tx) ([]T, error) {
		fRes, err2 := f(ctx, transaction)
		if err2 != nil {
			return nil, err2
		}
		if fRes == nil {
			return make([]T, 0), nil
		}
		return []T{*fRes}, nil
	})
	if err != nil || len(res) == 0 {
		return nil, err
	}
	return &res[0], err
}

type DBFunctionMultipleEntriesT[T any] func(ctx context.Context, transaction *sql.Tx) ([]T, error)

// WithTransactionMultipleEntriesT is the same as WithTransaction, but you can also return and array of data, not just the error.
func WithTransactionMultipleEntriesT[T any](h *DBHandler, ctx context.Context, f DBFunctionMultipleEntriesT[T]) ([]T, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
		// we ignore the error returned from Rollback() here,
		// because it is always set when Commit() was successful
	}(tx)

	result, err := f(ctx, tx)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	return result, nil
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
	EvtCreateEnvironmentApplicationLock EventType = "CreateEnvironmentApplicationLock"
	EvtDeleteEnvironmentApplicationLock EventType = "DeleteEnvironmentApplicationLock"
	EvtReleaseTrain                     EventType = "ReleaseTrain"
)

// DBWriteEslEventInternal writes one event to the event-sourcing-light table, taking arbitrary data as input
func (h *DBHandler) DBWriteEslEventInternal(ctx context.Context, eventType EventType, tx *sql.Tx, data interface{}) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery("INSERT INTO event_sourcing_light (created, event_type , json)  VALUES (?, ?, ?);")

	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		time.Now(),
		eventType,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write internal esl event into DB. Error: %w\n", err)
	}
	return nil
}

type EslEventRow struct {
	EslId     EslId
	Created   time.Time
	EventType EventType
	EventJson string
}

// DBReadEslEventInternal returns either the first or the last row of the esl table
func (h *DBHandler) DBReadEslEventInternal(ctx context.Context, tx *sql.Tx, firstRow bool) (*EslEventRow, error) {
	sort := "DESC"
	if firstRow {
		sort = "ASC"
	}
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslId, created, event_type , json FROM event_sourcing_light ORDER BY created %s LIMIT 1;", sort))
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
		EslId:     0,
		Created:   time.Unix(0, 0),
		EventType: "",
		EventJson: "",
	}
	if rows.Next() {
		err := rows.Scan(&row.EslId, &row.Created, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning event_sourcing_light row from DB. Error: %w\n", err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DBReadEslEventLaterThan returns the first row of the esl table that has an eslId > the given eslId
func (h *DBHandler) DBReadEslEventLaterThan(ctx context.Context, tx *sql.Tx, eslId EslId) (*EslEventRow, error) {
	sort := "ASC"
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT eslId, created, event_type, json FROM event_sourcing_light WHERE eslId > (?) ORDER BY created %s LIMIT 1;", sort))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		eslId,
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
		EslId:     0,
		Created:   time.Unix(0, 0),
		EventType: "",
		EventJson: "",
	}
	if !rows.Next() {
		row = nil
	} else {
		err := rows.Scan(&row.EslId, &row.Created, &row.EventType, &row.EventJson)
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

func (h *DBHandler) DBWriteAllApplications(ctx context.Context, transaction *sql.Tx, previousVersion int64, applications []string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllApplications")
	defer span.Finish()
	slices.Sort(applications) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllApplicationsJson{
		Apps: applications,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_apps (version , created , json)  VALUES (?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now(),
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not insert all apps into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) writeEvent(ctx context.Context, transaction *sql.Tx, eventuuid string, eventType event.EventType, sourceCommitHash string, eventJson []byte) error {
	span, _ := tracer.StartSpanFromContext(ctx, "writeEvent")
	defer span.Finish()
	insertQuery := h.AdaptQuery("INSERT INTO events (uuid, timestamp, commitHash, eventType, json)  VALUES (?, ?, ?, ?, ?);")

	rawUUID, err := timeuuid.ParseUUID(eventuuid)
	if err != nil {
		return fmt.Errorf("error parsing UUID. Error: %w", err)
	}
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		rawUUID.String(),
		uuid2.GetTime(&rawUUID).AsTime(),
		sourceCommitHash,
		eventType,
		eventJson)

	if err != nil {
		return fmt.Errorf("Error inserting event information into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteDeploymentEvent(ctx context.Context, transaction *sql.Tx, uuid, sourceCommitHash, email string, deployment *event.Deployment) error {
	metadata := event.Metadata{
		AuthorEmail: email,
		Uuid:        uuid,
	}
	jsonToInsert, err := json.Marshal(event.DBEventGo{
		EventData:     deployment,
		EventMetadata: metadata,
	})

	if err != nil {
		return fmt.Errorf("error marshalling deployment event to Json. Error: %v\n", err)
	}
	return h.writeEvent(ctx, transaction, uuid, event.EventTypeDeployment, sourceCommitHash, jsonToInsert)
}

func (h *DBHandler) DBSelectAllEventsForCommit(ctx context.Context, commitHash string) ([]EventRow, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEvents")
	defer span.Finish()

	query := h.AdaptQuery("SELECT uuid, timestamp, commitHash, eventType, json FROM events WHERE commitHash = (?) ORDER BY timestamp DESC LIMIT 100;")
	span.SetTag("query", query)

	rows, err := h.DB.QueryContext(ctx, query, commitHash)
	if err != nil {
		return nil, fmt.Errorf("Error querying events. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("events row could not be closed: %v", err)
		}
	}(rows)

	var result []EventRow

	for rows.Next() {
		var row = EventRow{
			Uuid:       "",
			Timestamp:  time.Unix(0, 0), //will be overwritten, prevents CI linter from complaining from missing fields
			CommitHash: "",
			EventType:  "",
			EventJson:  "",
		}
		err := rows.Scan(&row.Uuid, &row.Timestamp, &row.CommitHash, &row.EventType, &row.EventJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning events row from DB. Error: %w\n", err)
		}

		result = append(result, row)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("events: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("events: row has error: %v\n", err)
	}
	return result, nil
}

// DBSelectAllApplications returns (nil, nil) if there are no rows
func (h *DBHandler) DBSelectAllApplications(ctx context.Context, transaction *sql.Tx) (*AllApplicationsGo, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer span.Finish()
	query := "SELECT version, created, json FROM all_apps ORDER BY version DESC LIMIT 1;"
	span.SetTag("query", query)
	rows := transaction.QueryRowContext(ctx, query)
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
		return nil, fmt.Errorf("Error scanning all_apps row from DB. Error: %w\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("all_apps: row has error: %v\n", err)
	}

	//exhaustruct:ignore
	var resultJson = AllApplicationsJson{}
	err = json.Unmarshal(([]byte)(result.data), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("Error during json unmarshal of all_apps. Error: %w. Data: %s\n", err, result.data)
	}
	var resultGo = AllApplicationsGo{
		Version:             result.version,
		Created:             result.created,
		AllApplicationsJson: AllApplicationsJson{Apps: resultJson.Apps},
	}
	return &resultGo, nil
}

type DBDeployment struct {
	EslVersion     EslId
	Created        time.Time
	ReleaseVersion *int64
	App            string
	Env            string
	Metadata       string // json
}

type Deployment struct {
	EslVersion EslId
	Created    time.Time
	App        string
	Env        string
	Version    *int64
	Metadata   DeploymentMetadata
}

type DeploymentMetadata struct {
	DeployedByName  string
	DeployedByEmail string
}

type EnvironmentLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   EnvironmentLockMetadata
}

// DBEnvironmentLock Just used to fetch info from DB
type DBEnvironmentLock struct {
	EslVersion EslId
	Created    time.Time
	LockID     string
	Env        string
	Deleted    bool
	Metadata   string
}

type EnvironmentLockMetadata struct {
	CreatedByName  string
	CreatedByEmail string
	Message        string
}

type AllDeployments []Deployment
type AllEnvLocks map[string][]EnvironmentLock

type GetAllDeploymentsFun = func(ctx context.Context, transaction *sql.Tx) (AllDeployments, error)
type GetAllEnvLocksFun = func(ctx context.Context, transaction *sql.Tx) (AllEnvLocks, error)

// GetAllAppsFun returns a map where the Key is an app name, and the value is a team name of that app
type GetAllAppsFun = func() (map[string]string, error)

func (h *DBHandler) RunCustomMigrations(
	ctx context.Context,
	getAllAppsFun GetAllAppsFun,
	getAllDeploymentsFun GetAllDeploymentsFun,
	getAllEnvLocksFun GetAllEnvLocksFun,
) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	err := h.RunCustomMigrationAllAppsTable(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationApps(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationDeployments(ctx, getAllDeploymentsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvLocks(ctx, getAllEnvLocksFun)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBSelectDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string) (*Deployment, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectDeployment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, releaseVersion, appName, envName, metadata" +
			" FROM deployments " +
			" WHERE appName=? AND envName=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	var row = &DBDeployment{
		EslVersion:     0,
		Created:        time.Time{},
		ReleaseVersion: nil,
		App:            "",
		Env:            "",
		Metadata:       "",
	}
	var releaseVersion sql.NullInt64
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}
	if rows.Next() {
		err := rows.Scan(&row.EslVersion, &row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
		}

		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal in deployments. Error: %w. Data: %s\n", err, row.Metadata)
		}
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v\n", err)
	}
	return &Deployment{
		EslVersion: row.EslVersion,
		Created:    row.Created,
		App:        row.App,
		Env:        row.Env,
		Version:    row.ReleaseVersion,
		Metadata:   resultJson,
	}, nil
}

func (h *DBHandler) DBSelectAnyDeployment(ctx context.Context, tx *sql.Tx) (*DBDeployment, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, releaseVersion, appName, envName" +
			" FROM deployments " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &DBDeployment{}
	if rows.Next() {
		var releaseVersion sql.NullInt64
		err := rows.Scan(&row.EslVersion, &row.Created, &releaseVersion, &row.App, &row.Env)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
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

type DBApp struct {
	EslId EslId
	App   string
}

type DBAppMetaData struct {
	Team string
}

type DBAppWithMetaData struct {
	EslId       EslId
	App         string
	Metadata    DBAppMetaData
	StateChange AppStateChange
}

func (h *DBHandler) DBInsertApplication(ctx context.Context, transaction *sql.Tx, appName string, previousEslVersion EslId, stateChange AppStateChange, metaData DBAppMetaData) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBInsertApplication")
	defer span.Finish()
	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery(
		"INSERT INTO apps (eslVersion, created, appName, stateChange, metadata)  VALUES (?, ?, ?, ?, ?);",
	)
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now(),
		appName,
		stateChange,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not insert an app into DB. Error: %w\n", err)
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

func (h *DBHandler) DBSelectAnyApp(ctx context.Context, tx *sql.Tx) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, appName, metadata " +
			" FROM apps " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &DBAppWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.EslId, &row.App, &metadataStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning apps row from DB. Error: %w\n", err)
		}
		var metaData = DBAppMetaData{
			Team: "",
		}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of apps. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) DBSelectApp(ctx context.Context, tx *sql.Tx, appName string) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, appName, stateChange, metadata" +
			" FROM apps " +
			" WHERE appName=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row could not be closed: %v", err)
		}
	}(rows)

	//exhaustruct:ignore
	var row = &DBAppWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.EslId, &row.App, &row.StateChange, &metadataStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning apps row from DB. Error: %w\n", err)
		}
		var metaData = DBAppMetaData{Team: ""}
		err = json.Unmarshal(([]byte)(metadataStr), &metaData)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal of apps. Error: %w. Data: %s\n", err, metadataStr)
		}
		row.Metadata = metaData
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DBWriteDeployment writes one deployment, meaning "what should be deployed"
func (h *DBHandler) DBWriteDeployment(ctx context.Context, tx *sql.Tx, deployment Deployment, previousEslVersion EslId) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEslEventInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEslEventInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(deployment.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO deployments (eslVersion, created, releaseVersion, appName, envName, metadata) VALUES (?, ?, ?, ?, ?, ?);")

	span.SetTag("query", insertQuery)
	nullVersion := NewNullInt(deployment.Version)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		time.Now(),
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert)

	if err != nil {
		return fmt.Errorf("could not write deployment into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationDeployments(ctx context.Context, getAllDeploymentsFun GetAllDeploymentsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAnyDeployment(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}
		if allAppsDb != nil {
			l.Warnf("There are already deployments in the DB - skipping migrations")
			return nil
		}

		allDeploymentsInRepo, err := getAllDeploymentsFun(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get current deployments to run custom migrations: %v", err)
		}

		for i := range allDeploymentsInRepo {
			deploymentInRepo := allDeploymentsInRepo[i]
			err = h.DBWriteDeployment(ctx, transaction, deploymentInRepo, 0)
			if err != nil {
				return fmt.Errorf("error writing Deployment to DB for app %s in env %s: %v",
					deploymentInRepo.App, deploymentInRepo.Env, err)
			}
		}
		return nil
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
	Uuid       string
	Timestamp  time.Time
	CommitHash string
	EventType  event.EventType
	EventJson  string
}

func (h *DBHandler) RunCustomMigrationEnvLocks(ctx context.Context, getAllEnvLocksFun GetAllEnvLocksFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allEnvLocksDb, err := h.DBSelectAnyEnvLock(ctx, transaction)
		if err != nil {
			l.Infof("could not get environment locks from database - assuming the manifest repo is correct: %v", err)
			allEnvLocksDb = nil
		}
		if allEnvLocksDb != nil {
			l.Infof("There are already environment locks in the DB - skipping migrations")
			return nil
		}

		allEnvLocksInRepo, err := getAllEnvLocksFun(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get current environment locks to run custom migrations: %v", err)
		}

		for envName, locks := range allEnvLocksInRepo {
			var activeLockIds []string
			for _, currentLock := range locks {
				activeLockIds = append(activeLockIds, currentLock.LockID)
				err = h.DBWriteEnvironmentLockInternal(ctx, transaction, currentLock, 0, true)
				if err != nil {
					return fmt.Errorf("error writing environment locks to DB for environment %s: %v",
						envName, err)
				}
			}
			err := h.DBWriteAllEnvironmentLocks(ctx, transaction, 0, envName, activeLockIds)
			if err != nil {
				return fmt.Errorf("error writing environment locks ids to DB for environment %s: %v",
					envName, err)
			}
		}

		return nil
	})
}

func (h *DBHandler) RunCustomMigrationAllAppsTable(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}

		allAppsRepo, err := getAllAppsFun()
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
		sortedApps := sorting.SortKeys(allAppsRepo)

		if allAppsDb != nil && reflect.DeepEqual(allAppsDb.Apps, sortedApps) {
			// nothing to do
			logger.FromContext(ctx).Sugar().Infof("Nothing to do, all apps are equal")
			return nil
		}
		// if there is any difference, we assume the manifest wins over the database state,
		// so we use `allAppsRepo`:
		return h.DBWriteAllApplications(ctx, transaction, version, sortedApps)
	})
}

func (h *DBHandler) RunCustomMigrationApps(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
		dbApp, err := h.DBSelectAnyApp(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get dbApp from database - assuming the manifest repo is correct: %v", err)
		}
		if dbApp != nil {
			// the migration was already done
			logger.FromContext(ctx).Info("migration to apps was done already")
			return nil
		}

		appsMap, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get dbApp to run custom migrations: %v", err)
		}

		for app := range appsMap {
			team := appsMap[app]
			err = h.DBInsertApplication(ctx, transaction, app, InitialEslId, AppStateChangeMigrate, DBAppMetaData{Team: team})
			if err != nil {
				return fmt.Errorf("could not write dbApp %s: %v", app, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) DBSelectAnyEnvLock(ctx context.Context, tx *sql.Tx) (*DBEnvironmentLock, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
			" FROM environment_locks " +
			" LIMIT 1;"))
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
	var row = DBEnvironmentLock{}
	if rows.Next() {
		err := rows.Scan(&row.EslVersion, &row.Created, &row.LockID, &row.Env, &row.Metadata, &row.Deleted)
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
		return &row, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil // no rows, but also no error

}

func (h *DBHandler) DBSelectEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) (*EnvironmentLock, error) {
	selectQuery := h.AdaptQuery(fmt.Sprintf(
		"SELECT eslVersion, created, lockID, envName, metadata, deleted" +
			" FROM environment_locks " +
			" WHERE envName=? AND lockID=? " +
			" ORDER BY eslVersion DESC " +
			" LIMIT 1;"))

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
		var resultJson = EnvironmentLockMetadata{}
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

func (h *DBHandler) DBWriteEnvironmentLock(ctx context.Context, tx *sql.Tx, lockID, environment, message, authorName, authorEmail string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLock")
	defer span.Finish()

	var previousVersion EslId

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
		Metadata: EnvironmentLockMetadata{
			Message:        message,
			CreatedByName:  authorName,
			CreatedByEmail: authorEmail,
		},
		Deleted: false,
	}
	return h.DBWriteEnvironmentLockInternal(ctx, tx, envLock, previousVersion, false)
}

func (h *DBHandler) DBWriteEnvironmentLockInternal(ctx context.Context, tx *sql.Tx, envLock EnvironmentLock, previousEslVersion EslId, useTimeInLock bool) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLockInternal: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLockInternal")
	defer span.Finish()

	jsonToInsert, err := json.Marshal(envLock.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO environment_locks (eslVersion, created, lockID, envName, deleted, metadata) VALUES (?, ?, ?, ?, ?, ?);")

	var timetoInsert time.Time
	if useTimeInLock {
		timetoInsert = envLock.Created
	} else {
		timetoInsert = time.Now()
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		previousEslVersion+1,
		timetoInsert,
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
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectEnvLocks")
	defer span.Finish()

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
		var resultJson = EnvironmentLockMetadata{}
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

func (h *DBHandler) DBSelectAllEnvironmentLocks(ctx context.Context, tx *sql.Tx, environment string) (*AllEnvLocksGo, error) {
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvironmentLocks: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvironmentLocks")
	defer span.Finish()
	selectQuery := h.AdaptQuery(
		"SELECT version, created, environment, json FROM all_env_locks WHERE environment = ? ORDER BY version DESC LIMIT 1;")

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
	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvironmentLockSet: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentLockSet")
	defer span.Finish()

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
		//Get the latest change to
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
			var resultJson = EnvironmentLockMetadata{}
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
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteAllEnvironmentLocks")
	defer span.Finish()
	slices.Sort(lockIds) // we don't really *need* the sorting, it's just for convenience
	jsonToInsert, err := json.Marshal(AllEnvLocksJson{
		EnvLocks: lockIds,
	})
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	insertQuery := h.AdaptQuery("INSERT INTO all_env_locks (version , created, environment, json)  VALUES (?, ?, ?, ?);")
	span.SetTag("query", insertQuery)
	_, err = transaction.Exec(
		insertQuery,
		previousVersion+1,
		time.Now(),
		environment,
		jsonToInsert)
	if err != nil {
		return fmt.Errorf("could not insert all envs into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBDeleteEnvironmentLock(ctx context.Context, tx *sql.Tx, environment, lockID string) error {
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteEnvironmentLock: no transaction provided")
	}
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironmentLock")
	defer span.Finish()
	var previousVersion EslId

	//See if there is an existing lock with the same lock id in this environment. If it exists, just add a +1 to the eslversion
	existingEnvLock, err := h.DBSelectEnvironmentLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing environment lock: %w\n", err)
	}

	if existingEnvLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' does not exist", lockID, environment)
		return nil
	}

	if existingEnvLock.Deleted == true {
		logger.FromContext(ctx).Sugar().Warnf("could not delete lock. The environment lock '%s' on environment '%s' has already been deleted", lockID, environment)
		return nil
	} else {
		previousVersion = existingEnvLock.EslVersion
	}

	existingEnvLock.Deleted = true
	err = h.DBWriteEnvironmentLockInternal(ctx, tx, *existingEnvLock, previousVersion, false)

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
